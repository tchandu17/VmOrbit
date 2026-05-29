import { useState, useEffect } from "react";

/**
 * useDebounce delays updating the returned value until `delay` ms have passed
 * since the last change to `value`. Use this to avoid firing API calls on
 * every keystroke in search/filter inputs.
 */
export function useDebounce<T>(value: T, delay = 300): T {
  const [debounced, setDebounced] = useState<T>(value);

  useEffect(() => {
    const timer = setTimeout(() => setDebounced(value), delay);
    return () => clearTimeout(timer);
  }, [value, delay]);

  return debounced;
}
