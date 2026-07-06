// Package embedder provides implementations of domain.Embedder.
// BM25Embedder produces sparse pseudo-embeddings using BM25 term weights
// over a shared vocabulary built from indexed documents.
package embedder

import (
	"context"
	"math"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/domain"
)

// Compile-time assertion: BM25Embedder implements domain.Embedder.
var _ domain.Embedder = (*BM25Embedder)(nil)

// BM25Parameters holds tuning knobs for BM25 scoring.
type BM25Parameters struct {
	K1 float64 // term-frequency saturation (default 1.5)
	B  float64 // document-length normalisation (default 0.75)
}

// DefaultBM25Parameters returns the standard Okapi BM25 defaults.
func DefaultBM25Parameters() BM25Parameters {
	return BM25Parameters{K1: 1.5, B: 0.75}
}

// tokenRe splits text into lowercase alphanumeric tokens.
var tokenRe = regexp.MustCompile(`[^a-z0-9]+`)

// tokenize lowercases text and returns alphanumeric tokens.
func tokenize(text string) []string {
	parts := tokenRe.Split(strings.ToLower(text), -1)
	out := parts[:0]
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// BM25Embedder implements domain.Embedder using BM25 pseudo-embeddings.
//
// Call Index to populate the corpus before calling Embed. The vector
// dimension equals the vocabulary size (capped at maxDims).
//
// BM25 parameters: k1 = 1.5, b = 0.75 (via DefaultBM25Parameters).
type BM25Embedder struct {
	mu      sync.Mutex
	params  BM25Parameters
	maxDims int

	// Corpus kept for IDF computation.
	docs     map[string][]string // docID -> tokens
	df       map[string]int      // term -> document frequency
	totalLen int                 // sum of all document token counts

	// Built vocabulary (rebuilt lazily when dirty).
	vocab    []string
	vocabIdx map[string]int
	dirty    bool
}

// NewBM25Embedder creates an embedder with the supplied parameters.
// maxDims caps the vocabulary size; pass 0 to use the default of 4096.
func NewBM25Embedder(params BM25Parameters, maxDims int) *BM25Embedder {
	if maxDims <= 0 {
		maxDims = 4096
	}
	return &BM25Embedder{
		params:  params,
		maxDims: maxDims,
		docs:    make(map[string][]string),
		df:      make(map[string]int),
		dirty:   true,
	}
}

// New returns a BM25Embedder with default parameters and a 4096-dimension cap.
func New() *BM25Embedder {
	return NewBM25Embedder(DefaultBM25Parameters(), 4096)
}

// Index adds or replaces a document in the corpus for IDF computation.
// Safe to call concurrently with other Index calls.
func (e *BM25Embedder) Index(id, text string) {
	tokens := tokenize(text)

	e.mu.Lock()
	defer e.mu.Unlock()

	// Remove old document's contribution to df and total length.
	if old, ok := e.docs[id]; ok {
		seen := make(map[string]struct{}, len(old))
		for _, t := range old {
			if _, dup := seen[t]; !dup {
				e.df[t]--
				if e.df[t] == 0 {
					delete(e.df, t)
				}
				seen[t] = struct{}{}
			}
		}
		e.totalLen -= len(old)
	}

	// Register new document.
	e.docs[id] = tokens
	e.totalLen += len(tokens)

	seen := make(map[string]struct{}, len(tokens))
	for _, t := range tokens {
		if _, dup := seen[t]; !dup {
			e.df[t]++
			seen[t] = struct{}{}
		}
	}

	e.dirty = true
}

// buildVocab rebuilds the vocabulary from the current corpus.
// Selects up to maxDims terms ordered by IDF descending (lowest document
// frequency first = highest discriminativeness), breaking ties alphabetically.
// Caller must hold e.mu.
func (e *BM25Embedder) buildVocab() {
	type termFreq struct {
		term string
		freq int
	}
	terms := make([]termFreq, 0, len(e.df))
	for t, f := range e.df {
		if f > 0 {
			terms = append(terms, termFreq{t, f})
		}
	}
	// Sort by DF ascending (low DF = high IDF = most discriminative) so the
	// vocabulary captures the terms that most distinguish documents from each other.
	sort.Slice(terms, func(i, j int) bool {
		if terms[i].freq != terms[j].freq {
			return terms[i].freq < terms[j].freq // ascending DF
		}
		return terms[i].term < terms[j].term
	})
	if len(terms) > e.maxDims {
		terms = terms[:e.maxDims]
	}

	e.vocab = make([]string, len(terms))
	e.vocabIdx = make(map[string]int, len(terms))
	for i, tv := range terms {
		e.vocab[i] = tv.term
		e.vocabIdx[tv.term] = i
	}
	e.dirty = false
}

// Dims returns the maximum vocabulary capacity, which is the fixed embedding
// dimension produced by Embed. Using maxDims (rather than len(vocab)) ensures
// the vector store can be constructed with a stable dimension before any
// corpus documents are indexed.
func (e *BM25Embedder) Dims() int {
	return e.maxDims
}

// Embed returns a BM25 score vector for each input text.
//
// Each returned vector has length Dims(). Element i is the BM25 weight of
// vocabulary term i within the given text, using corpus-level IDF. The
// VectorStore is expected to use cosine similarity for retrieval.
//
// If the corpus is empty or a text contains no vocabulary terms, its vector
// is all-zeros.
func (e *BM25Embedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	e.mu.Lock()
	if e.dirty {
		e.buildVocab()
	}

	// Snapshot corpus-level stats so computation can run without holding the lock.
	n := len(e.docs)
	avgdl := 0.0
	if n > 0 {
		avgdl = float64(e.totalLen) / float64(n)
	}
	_ = e.vocab // vocab is not used directly; vocabIdx provides the term→index mapping
	vocabIdx := e.vocabIdx
	dfSnap := make(map[string]int, len(e.df))
	for t, f := range e.df {
		dfSnap[t] = f
	}
	k1 := e.params.K1
	b := e.params.B
	e.mu.Unlock()

	result := make([][]float32, len(texts))
	for i, text := range texts {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		tokens := tokenize(text)
		docLen := float64(len(tokens))

		tf := make(map[string]int, len(tokens))
		for _, t := range tokens {
			tf[t]++
		}

		// Always allocate maxDims elements so that all vectors share the same
		// fixed dimension regardless of the current vocabulary size. Positions
		// for out-of-vocabulary terms remain zero, which is correct for cosine
		// similarity: absent terms contribute nothing to the dot product.
		vec := make([]float32, e.maxDims)
		for term, idx := range vocabIdx {
			termTF := tf[term]
			if termTF == 0 {
				continue
			}
			termDF := dfSnap[term]

			// Okapi BM25 IDF: ln( (N - df + 0.5) / (df + 0.5) + 1 )
			numerator := float64(n-termDF) + 0.5
			denominator := float64(termDF) + 0.5
			idf := math.Log(numerator/denominator + 1.0)
			if idf < 0 {
				idf = 0
			}

			// BM25 TF normalisation.
			var normTF float64
			if avgdl > 0 {
				normTF = float64(termTF) * (k1 + 1) /
					(float64(termTF) + k1*(1-b+b*docLen/avgdl))
			} else {
				// No corpus length baseline; use unsaturated TF.
				normTF = float64(termTF) * (k1 + 1) / (float64(termTF) + k1)
			}

			vec[idx] = float32(idf * normTF)
		}
		result[i] = vec
	}
	return result, nil
}
