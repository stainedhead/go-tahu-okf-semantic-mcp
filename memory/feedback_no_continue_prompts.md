---
name: feedback-no-continue-prompts
description: User wants autonomous execution — never stop mid-flow asking for "continue" or confirmation between steps
metadata:
  type: feedback
---

Do not pause between orchestrated dev-flow steps asking the user to type "continue". Once a multi-step skill (implm-frm-prd, implm-from-spec, etc.) is running, execute all steps to completion autonomously without stopping between steps.

**Why:** User explicitly said "keep running to the end, you should not be asking for these continue statements from me."

**How to apply:** In dev-flow step orchestration, update the dashboard, chain directly to the next step's skill or implementation without yielding to the user between steps. Only stop if a step fails (❌ Failed) or a genuinely ambiguous user-decision is required.
