#!/usr/bin/env node
/**
 * Simple stdio worker example for Orkes Conductor CLI.
 *
 * This worker reads a task from stdin, processes it, and returns a result to stdout.
 */

// Read task from stdin
let input = '';
process.stdin.setEncoding('utf8');

process.stdin.on('data', chunk => {
  input += chunk;
});

process.stdin.on('end', () => {
  try {
    // Parse task JSON
    const task = JSON.parse(input);
    const inputData = task.inputData || {};
    const name = inputData.name || 'World';

    // Get task metadata
    const taskId = task.taskId || 'unknown';
    const workflowId = task.workflowInstanceId || 'unknown';
    const taskType = task.taskType || 'unknown';

    // Process the task
    const message = `Hello, ${name}!`;

    // Return result to stdout
    const result = {
      status: 'COMPLETED',
      output: {
        message: message,
        taskId: taskId,
        workflowId: workflowId
      },
      logs: [
        `Processing task ${taskId} of type ${taskType}`,
        `Workflow: ${workflowId}`,
        `Generated greeting for ${name}`
      ]
    };

    console.log(JSON.stringify(result));

  } catch (error) {
    // Return failure result on error
    const result = {
      status: 'FAILED',
      reason: error.message,
      logs: [`Error processing task: ${error.message}`]
    };
    console.log(JSON.stringify(result));
  }
});
