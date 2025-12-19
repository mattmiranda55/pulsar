<script>
  import { output, isRunning } from '../stores/app.js';
</script>

<div class="output-container">
  <div class="output-header">
    <span>Output</span>
    {#if $isRunning}
      <span class="running">Running...</span>
    {/if}
  </div>
  <pre class="output-content">{$output || 'Run your code to see output here...'}</pre>
</div>

<style>
  .output-container {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    background: #1e1e1e;
    border-left: 1px solid #333;
  }

  .output-header {
    padding: 8px 16px;
    background: #252526;
    border-bottom: 1px solid #333;
    font-size: 12px;
    color: #888;
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .running {
    color: #0078d4;
  }

  .output-content {
    flex: 1;
    margin: 0;
    padding: 16px;
    overflow: auto;
    font-family: "Reddit Mono Variable", monospace;
    font-size: 13px;
    color: #d4d4d4;
    white-space: pre-wrap;
    word-wrap: break-word;
  }
</style>
