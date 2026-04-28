// Simple script to analyze K6 test results
// Usage: node analyze-results.js <summary-file.json>

const fs = require('fs');
const path = require('path');

function analyzeResults(summaryFile) {
  if (!fs.existsSync(summaryFile)) {
    console.error(`Error: File not found: ${summaryFile}`);
    process.exit(1);
  }

  const data = JSON.parse(fs.readFileSync(summaryFile, 'utf8'));
  const metrics = data.metrics;

  console.log('\n========================================');
  console.log('K6 Test Results Analysis');
  console.log('========================================\n');

  // Test Information
  console.log('Test Information:');
  console.log(`  File: ${summaryFile}`);
  console.log(`  Root Group: ${data.root_group.name || 'N/A'}`);
  console.log('');

  // HTTP Metrics
  console.log('HTTP Metrics:');
  
  if (metrics.http_reqs) {
    console.log(`  Total Requests: ${metrics.http_reqs.values.count}`);
    console.log(`  Request Rate: ${metrics.http_reqs.values.rate.toFixed(2)} req/s`);
  }

  if (metrics.http_req_duration) {
    console.log(`  Avg Response Time: ${metrics.http_req_duration.values.avg.toFixed(2)}ms`);
    console.log(`  Min Response Time: ${metrics.http_req_duration.values.min.toFixed(2)}ms`);
    console.log(`  Max Response Time: ${metrics.http_req_duration.values.max.toFixed(2)}ms`);
    console.log(`  P50 Response Time: ${metrics.http_req_duration.values.med.toFixed(2)}ms`);
    console.log(`  P95 Response Time: ${metrics.http_req_duration.values['p(95)'].toFixed(2)}ms`);
    console.log(`  P99 Response Time: ${metrics.http_req_duration.values['p(99)'].toFixed(2)}ms`);
  }

  if (metrics.http_req_failed) {
    const failRate = (metrics.http_req_failed.values.rate * 100).toFixed(2);
    console.log(`  Failed Requests: ${metrics.http_req_failed.values.passes} (${failRate}%)`);
  }

  console.log('');

  // Virtual Users
  console.log('Virtual Users:');
  if (metrics.vus) {
    console.log(`  Min VUs: ${metrics.vus.values.min}`);
    console.log(`  Max VUs: ${metrics.vus.values.max}`);
  }
  if (metrics.vus_max) {
    console.log(`  Max VUs Allocated: ${metrics.vus_max.values.max}`);
  }
  console.log('');

  // Iterations
  console.log('Iterations:');
  if (metrics.iterations) {
    console.log(`  Total Iterations: ${metrics.iterations.values.count}`);
    console.log(`  Iteration Rate: ${metrics.iterations.values.rate.toFixed(2)} iter/s`);
    console.log(`  Avg Iteration Duration: ${metrics.iteration_duration.values.avg.toFixed(2)}ms`);
  }
  console.log('');

  // Data Transfer
  console.log('Data Transfer:');
  if (metrics.data_received) {
    const dataReceived = (metrics.data_received.values.count / 1024 / 1024).toFixed(2);
    console.log(`  Data Received: ${dataReceived} MB`);
    console.log(`  Receive Rate: ${(metrics.data_received.values.rate / 1024).toFixed(2)} KB/s`);
  }
  if (metrics.data_sent) {
    const dataSent = (metrics.data_sent.values.count / 1024 / 1024).toFixed(2);
    console.log(`  Data Sent: ${dataSent} MB`);
    console.log(`  Send Rate: ${(metrics.data_sent.values.rate / 1024).toFixed(2)} KB/s`);
  }
  console.log('');

  // Thresholds
  console.log('Thresholds:');
  const thresholds = data.thresholds || {};
  let allPassed = true;
  
  Object.keys(thresholds).forEach(key => {
    const threshold = thresholds[key];
    Object.keys(threshold).forEach(condition => {
      const passed = threshold[condition].ok;
      const status = passed ? '✓' : '✗';
      console.log(`  ${status} ${key}: ${condition}`);
      if (!passed) allPassed = false;
    });
  });
  
  console.log('');

  // Overall Result
  console.log('========================================');
  if (allPassed) {
    console.log('Overall Result: ✓ PASSED');
  } else {
    console.log('Overall Result: ✗ FAILED');
  }
  console.log('========================================\n');

  // Recommendations
  console.log('Recommendations:');
  
  if (metrics.http_req_duration && metrics.http_req_duration.values['p(95)'] > 1000) {
    console.log('  ⚠ P95 response time is high (>1s). Consider optimizing slow endpoints.');
  }
  
  if (metrics.http_req_failed && metrics.http_req_failed.values.rate > 0.1) {
    console.log('  ⚠ Error rate is high (>10%). Check server logs and error responses.');
  }
  
  if (metrics.http_req_duration && metrics.http_req_duration.values.max > 5000) {
    console.log('  ⚠ Maximum response time is very high (>5s). Investigate timeout issues.');
  }
  
  if (metrics.http_reqs && metrics.http_reqs.values.rate < 10) {
    console.log('  ℹ Low request rate. Consider increasing virtual users for more load.');
  }

  console.log('');

  return allPassed ? 0 : 1;
}

// Main
if (require.main === module) {
  const args = process.argv.slice(2);
  
  if (args.length === 0) {
    console.log('Usage: node analyze-results.js <summary-file.json>');
    console.log('');
    console.log('Example:');
    console.log('  node analyze-results.js results/20240101_120000/load-summary.json');
    process.exit(1);
  }

  const summaryFile = args[0];
  const exitCode = analyzeResults(summaryFile);
  process.exit(exitCode);
}

module.exports = { analyzeResults };
