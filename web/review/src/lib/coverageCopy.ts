// The words for coverage, in one place. In a blind evaluation a reviewer skipped files
// "because the AI's reviewers had traced these" and missed a real defect there: "the AI
// examined it" was read as "the AI verified it". So the AI's coverage is always "looked
// at", always carries this disclaimer, and only the human's own review counts as verified.

/** Tooltip / legend text wherever the AI's coverage appears */
export const AI_LOOKED_HINT =
  "The AI opened this code. That doesn't mean it's correct. Only findings are claims, and nothing here is verified until you've checked it.";

/** Tooltip for the human's coverage */
export const YOU_REVIEWED_HINT =
  "What you've viewed yourself (on screen for 2 seconds). This is the only coverage that counts as verified.";

/** Short qualifier shown next to AI coverage numbers */
export const AI_NOT_VERIFIED = 'not verified';
