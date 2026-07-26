import { performance } from 'node:perf_hooks';

const queueSize = Number(process.env.QUEUE_SIZE ?? 1000);
const iterations = Number(process.env.ITERATIONS ?? 1000);
let queue = Array.from({ length: queueSize }, (_, index) => ({ queueId: `q-${index}`, name: `Track ${index}` }));

const startedAt = performance.now();
for (let index = 0; index < iterations; index += 1) {
  const item = { queueId: `new-${index}`, name: `New ${index}` };
  queue = [...queue, item];
  queue = queue.filter(entry => entry.queueId !== `new-${Math.max(0, index - 1)}`);
}

console.log(JSON.stringify({
  queueSize,
  iterations,
  elapsedMs: Number((performance.now() - startedAt).toFixed(2)),
  finalQueueSize: queue.length
}, null, 2));
