// Application state: the Kaizen review context (static) plus live findings,
// engine status and selection. Views subscribe to named change events.

export function createStore(context) {
  const listeners = new Map();
  const files = new Map(context.files.map((file) => [file.path, file]));
  const functions = new Map(context.functions.map((fn) => [fn.id, fn]));
  const nodes = new Map(context.graph.nodes.map((node) => [node.id, node]));

  const functionsByFile = new Map();
  for (const fn of context.functions) {
    if (!functionsByFile.has(fn.file)) functionsByFile.set(fn.file, []);
    functionsByFile.get(fn.file).push(fn);
  }
  for (const list of functionsByFile.values()) list.sort((left, right) => left.start_line - right.start_line);

  const store = {
    context,
    files,
    functions,
    nodes,
    functionsByFile,
    findings: new Map(),
    freshFindingIds: new Set(),
    included: new Set(),
    result: null,
    status: { state: 'connecting', message: 'Connecting', agent: '' },
    activity: [],
    selection: { functionId: null, nodeId: null, file: null },

    on(name, callback) {
      if (!listeners.has(name)) listeners.set(name, []);
      listeners.get(name).push(callback);
    },

    emit(name, payload) {
      for (const callback of listeners.get(name) || []) callback(payload);
    },

    findingsList() {
      return [...store.findings.values()];
    },

    findingsForFile(path) {
      return store.findingsList().filter((finding) => finding.file === path);
    },

    findingsForFunction(fn) {
      return store.findingsList().filter((finding) => findingBelongsTo(finding, fn));
    },

    /** The changed function that contains a head-side line of a file */
    functionAtLine(path, line) {
      return (functionsByFile.get(path) || []).find((fn) => line >= fn.start_line && line <= fn.end_line) || null;
    },

    addFinding(finding) {
      if (!finding || !finding.id) return;
      const isNew = !store.findings.has(finding.id);
      store.findings.set(finding.id, finding);
      if (isNew) {
        store.freshFindingIds.add(finding.id);
        if (finding.severity !== 'praise') store.included.add(finding.id);
        window.setTimeout(() => store.freshFindingIds.delete(finding.id), 1500);
      }
      store.emit('finding', { finding, isNew });
    },

    setIncluded(id, included) {
      if (included) store.included.add(id);
      else store.included.delete(id);
      store.emit('included', { id, included });
    },

    setResult(result) {
      store.result = result;
      for (const finding of result.findings || []) store.addFinding(finding);
      store.emit('result', result);
    },

    setStatus(status) {
      store.status = { ...store.status, ...status };
      store.emit('status', store.status);
    },

    addActivity(item) {
      store.activity.push(item);
      store.emit('activity', item);
    },

    /** True when a finding sits on a head-side line shown in the diff (it can be an inline comment) */
    isInlineFinding(finding) {
      const file = files.get(finding.file);
      const line = finding.end_line || finding.line;
      if (!file || !finding.line) return false;
      return file.hunks.some((hunk) => hunk.lines.some((diffLine) => diffLine.kind !== 'removed' && diffLine.new_line === line));
    },
  };
  return store;
}

function findingBelongsTo(finding, fn) {
  if (finding.function_id) return finding.function_id === fn.id;
  return finding.file === fn.file && finding.line >= fn.start_line && finding.line <= fn.end_line;
}
