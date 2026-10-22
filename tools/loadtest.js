#!/usr/bin/env node
// Tiny load tester (Node 18+, no dependencies).
//   node tools/loadtest.js [url] [requests] [concurrency]
// Example: node tools/loadtest.js http://localhost:8080/api/tasks?q=deploy 500 20

const url = process.argv[2] || "http://localhost:8080/api/tasks?page=1&limit=20";
const total = parseInt(process.argv[3]) || 200;
const concurrency = parseInt(process.argv[4]) || 10;

const latencies = [];
let failures = 0;

async function hit() {
  const start = Date.now();
  const res = await fetch(url);
  await res.text();
  latencies.push(Date.now() - start);
}

function percentile(sorted, p) {
  return sorted[Math.floor(sorted.length * p)];
}

async function main() {
  console.log(`GET ${url}  total=${total} concurrency=${concurrency}`);
  const began = Date.now();

  for (let i = 0; i < total; i++) {
    try {
      await hit();
    } catch (err) {
      failures++;
    }
  }

  const secs = (Date.now() - began) / 1000;
  latencies.sort();
  console.log(`done in ${secs.toFixed(2)}s  (${(total / secs).toFixed(1)} req/s)`);
  console.log(`failures: ${failures}`);
  console.log(`p50=${percentile(latencies, 0.5)}ms  p95=${percentile(latencies, 0.95)}ms  p100=${percentile(latencies, 1)}ms`);
}

main();
