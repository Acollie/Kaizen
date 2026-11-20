// Wire types for the review UI. These mirror the JSON produced by pkg/review/types.go
// (and the metrics/concern structs from pkg/models) exactly; keep them in sync.

export interface PullRequest {
  number: number;
  title: string;
  body: string;
  author: string;
  url: string;
  owner: string;
  repo: string;
  base_ref: string;
  head_ref: string;
  base_sha: string;
  head_sha: string;
  merge_base: string;
  additions: number;
  deletions: number;
  file_count: number;
}

export type FileStatus = 'added' | 'modified' | 'deleted' | 'renamed';
export type DiffLineKind = 'context' | 'added' | 'removed';

export interface DiffLine {
  kind: DiffLineKind;
  /** 1-based; 0 when the line does not exist on the base side */
  old_line: number;
  /** 1-based; 0 when the line does not exist on the head side (removals) */
  new_line: number;
  text: string;
}

export interface Hunk {
  old_start: number;
  old_count: number;
  new_start: number;
  new_count: number;
  /** Text after the @@ markers, often the enclosing function */
  header: string;
  lines: DiffLine[];
}

export interface FileDiff {
  path: string;
  old_path: string;
  status: FileStatus;
  /** "Go", "Python", "Kotlin", "Swift", or "" */
  language: string;
  additions: number;
  deletions: number;
  binary: boolean;
  /** Lockfile or minified output: hunks are omitted (see risk_reasons) */
  generated?: boolean;
  hunks: Hunk[];
  /** Total lines in the head version (bounds for context expansion) */
  line_count: number;
  risk_score: number;
  risk_reasons?: string[];
}

export interface FunctionRef {
  id: string;
  name: string;
  file: string;
  /** Call-site line for callers, definition line for callees */
  line: number;
  external: boolean;
}

export interface ChurnMetric {
  total_commits?: number;
  [key: string]: unknown;
}

/** models.FunctionAnalysis (subset the UI shows; other fields pass through) */
export interface FunctionMetrics {
  name: string;
  start_line: number;
  end_line: number;
  length: number;
  logical_lines: number;
  parameter_count: number;
  cyclomatic_complexity: number;
  cognitive_complexity: number;
  nesting_depth: number;
  halstead_volume: number;
  maintainability_index?: number;
  fan_in: number;
  fan_out: number;
  churn?: ChurnMetric | null;
  is_hotspot?: boolean;
  [key: string]: unknown;
}

export interface FunctionRisk {
  id: string;
  name: string;
  receiver_type?: string;
  file: string;
  language: string;
  start_line: number;
  end_line: number;
  lines_added: number;
  lines_removed: number;
  is_new: boolean;
  metrics?: FunctionMetrics | null;
  fan_in: number;
  fan_in_approximate: boolean;
  callers: FunctionRef[] | null;
  callees: FunctionRef[] | null;
  risk_score: number;
  risk_reasons: string[] | null;
  focus: boolean;
}

export type GraphNodeKind = 'changed' | 'caller' | 'callee' | 'external';

export interface GraphNode {
  id: string;
  name: string;
  file: string;
  line: number;
  kind: GraphNodeKind;
  risk_score: number;
  focus: boolean;
  fan_in: number;
  complexity: number;
}

export interface GraphEdge {
  from: string;
  to: string;
  weight: number;
}

export interface ReviewGraph {
  nodes: GraphNode[] | null;
  edges: GraphEdge[] | null;
}

export interface AffectedItem {
  file_path: string;
  function_name?: string;
  line?: number;
  metrics: Record<string, number> | null;
}

export interface Concern {
  type: string;
  severity: string;
  title: string;
  description: string;
  affected_items: AffectedItem[] | null;
}

export interface ReviewContext {
  pr: PullRequest;
  files: FileDiff[] | null;
  functions: FunctionRisk[] | null;
  graph: ReviewGraph;
  concerns: Concern[] | null;
  worktree_path: string;
  generated_at: string;
}

export type Severity = 'critical' | 'warning' | 'suggestion' | 'praise';
export type Category = 'correctness' | 'security' | 'performance' | 'design' | 'tests';

export interface Finding {
  id: string;
  severity: Severity;
  category: Category;
  file: string;
  /** Head-side line; 0/absent means file- or PR-level */
  line?: number;
  end_line?: number;
  function_id?: string;
  title: string;
  /** Markdown */
  body: string;
  suggestion?: string;
  /** One concrete step a developer can take to confirm the finding themselves */
  verify?: string;
  confidence: number;
  /** Agent that produced it, e.g. "hotspot-reviewer" */
  source?: string;
}

export type Verdict = 'approve' | 'comment' | 'request_changes';

export interface ReviewResult {
  summary: string;
  verdict: Verdict;
  findings: Finding[] | null;
  engine: string;
  model?: string;
  cost_usd?: number;
}

export type EventType = 'status' | 'thinking' | 'tool_call' | 'finding' | 'result' | 'error' | 'done';

export interface StatusData { message: string; agent?: string }
export interface TextData { text: string; agent?: string }
export interface ToolCallData {
  tool: string;
  summary: string;
  agent?: string;
  /**
   * What the tool looked at, when known: a function ID (get_function, get_callers,
   * get_callees) or a repo-relative file path (get_file_diff, read_lines, Read).
   * Used to show which changed code the AI examined and which it never opened.
   */
  target?: string;
  target_kind?: 'function' | 'file';
}

interface EventBase<T extends EventType, D> {
  seq: number;
  type: T;
  time: string;
  data: D;
}

export type ReviewEvent =
  | EventBase<'status', StatusData>
  | EventBase<'thinking', TextData>
  | EventBase<'tool_call', ToolCallData>
  | EventBase<'finding', Finding>
  | EventBase<'result', ReviewResult>
  | EventBase<'error', TextData>
  | EventBase<'done', null | undefined>;

export interface PostRequest {
  summary: string;
  findings: Finding[];
  /** Required to post again after this session already posted a review (else 409) */
  confirm_repost?: boolean;
}

export interface PostResult {
  url: string;
  inline_comments: number;
  body_findings: number;
  /** GitHub rejected an inline comment's line, so every finding was posted in the body */
  fallback_to_body?: boolean;
}

/** GET /api/file response */
export interface FileSlice {
  lines: { n: number; text: string }[];
  total: number;
}
