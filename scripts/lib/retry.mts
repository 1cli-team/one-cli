type RetryPolicy = {
  delays: readonly number[];
  shouldRetry: (error: unknown) => boolean;
  onRetry: (error: unknown, delay: number, attempt: number) => void;
};

export function retry<T>(operation: () => T, policy: RetryPolicy): T {
  for (let attempt = 0; ; attempt++) {
    try {
      return operation();
    } catch (error) {
      const delay = policy.delays[attempt];
      if (delay === undefined || !policy.shouldRetry(error)) throw error;
      policy.onRetry(error, delay, attempt + 1);
      Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, delay);
    }
  }
}
