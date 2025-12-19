<script>
  import '@fontsource-variable/reddit-mono';
  import '@fontsource-variable/nunito';
  import Sidebar from './components/Sidebar.svelte';
  import Editor from './components/Editor.svelte';
  import Output from './components/Output.svelte';
  import Toolbar from './components/Toolbar.svelte';
  import { layout } from './stores/app.js';
</script>

<main class="app">
  <Sidebar />
  <div class="main-content">
    <Toolbar />
    <div class="editor-output" class:vertical={$layout === 'vertical'}>
      <Editor />
      <Output />
    </div>
  </div>
</main>

<style>
  .app {
    display: flex;
    height: 100vh;
    width: 100vw;
    overflow: hidden;
  }

  .main-content {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-width: 0;
  }

  .editor-output {
    flex: 1;
    display: flex;
    min-height: 0;
  }

  .editor-output.vertical {
    flex-direction: column;
  }

  .editor-output.vertical :global(.output-container) {
    border-left: none;
    border-top: 1px solid #333;
  }
</style>
