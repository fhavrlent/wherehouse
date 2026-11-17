import { useRef, type CompositionEvent, type KeyboardEvent } from "react";

/**
 * Tells an Enter that confirms a character being composed with an input
 * method (Japanese, Chinese, Korean) from one meant to act. Safari reports
 * composing as over just before that Enter arrives, so an Enter right after
 * composing ended counts as part of it too.
 */
export const useComposing = () => {
  const endedRef = useRef<number | null>(null);
  return {
    onCompositionEnd: (e: CompositionEvent) => {
      endedRef.current = e.timeStamp;
    },
    composing: (e: KeyboardEvent) =>
      e.nativeEvent.isComposing || (endedRef.current !== null && Math.abs(e.timeStamp - endedRef.current) < 50),
  };
};
