// Lookup maps over the (immutable) review context, built once at startup.

import type { FileDiff, FunctionRisk, GraphNode, ReviewContext } from '../types';

export interface ReviewIndex {
  files: Map<string, FileDiff>;
  functions: Map<string, FunctionRisk>;
  nodes: Map<string, GraphNode>;
  /** Changed functions per file, in line order */
  functionsByFile: Map<string, FunctionRisk[]>;
}

export function buildIndex(context: ReviewContext): ReviewIndex {
  const files = new Map((context.files ?? []).map((file) => [file.path, file]));
  const functions = new Map((context.functions ?? []).map((fn) => [fn.id, fn]));
  const nodes = new Map((context.graph.nodes ?? []).map((node) => [node.id, node]));
  const functionsByFile = new Map<string, FunctionRisk[]>();
  for (const fn of context.functions ?? []) {
    const list = functionsByFile.get(fn.file) ?? [];
    list.push(fn);
    functionsByFile.set(fn.file, list);
  }
  for (const list of functionsByFile.values()) list.sort((left, right) => left.start_line - right.start_line);
  return { files, functions, nodes, functionsByFile };
}
