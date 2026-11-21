// Always-mounted registration of g r (gotoCallers), so it works from any tab. Mount
// once next to the tab body (e.g. in ContextPanel). The Function and Flow panels also
// register it while visible; the request is idempotent, so duplicates are harmless.

import { useReviewStore } from '../../state/StoreProvider';
import { useCommands } from '../../state/commands';
import { requestCallersFocus } from './callersRequest';

export function FunctionCommands() {
  const store = useReviewStore();
  useCommands({ gotoCallers: () => requestCallersFocus(store) });
  return null;
}
