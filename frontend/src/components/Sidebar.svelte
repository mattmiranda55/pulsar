<script>
  import { onMount } from 'svelte';
  import { projects, currentProject } from '../stores/app.js';
  import { GetProjects, AddProject, RemoveProject, SelectDirectory } from '../../wailsjs/go/main/App.js';

  onMount(async () => {
    const savedProjects = await GetProjects();
    projects.set(savedProjects || []);
    if (savedProjects && savedProjects.length > 0) {
      currentProject.set(savedProjects[0]);
    }
  });

  async function addProject() {
    try {
      const path = await SelectDirectory();
      
      if (!path) return;
      
      const name = prompt('Enter project name:', path.split('/').pop());
      if (!name) return;

      const project = await AddProject(name, path);
      projects.update(p => [...p, project]);
      currentProject.set(project);
    } catch (err) {
      alert(err);
    }
  }

  async function removeProject(e, project) {
    e.stopPropagation();
    if (confirm(`Remove "${project.name}" from projects?`)) {
      await RemoveProject(project.id);
      projects.update(p => p.filter(pr => pr.id !== project.id));
      if ($currentProject?.id === project.id) {
        currentProject.set($projects[0] || null);
      }
    }
  }
</script>

<aside class="sidebar">
  <button class="add-project" on:click={addProject} title="Add Laravel Project">
    <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      <path d="M12 5v14M5 12h14"/>
    </svg>
  </button>
  
  <div class="projects">
    {#each $projects as project}
      <button 
        class="project-btn" 
        class:active={$currentProject?.id === project.id}
        on:click={() => currentProject.set(project)}
        on:contextmenu|preventDefault={(e) => removeProject(e, project)}
        title={`${project.name}\n${project.path}`}
      >
        {project.name.charAt(0).toUpperCase()}
      </button>
    {/each}
  </div>

  {#if $projects.length === 0}
    <div class="no-projects">
      <span>Add a Laravel project to get started</span>
    </div>
  {/if}
</aside>

<style>
  .sidebar {
    width: 48px;
    background: #1e1e1e;
    border-right: 1px solid #333;
    display: flex;
    flex-direction: column;
    align-items: center;
    padding: 8px 0;
    gap: 8px;
  }

  .add-project {
    width: 36px;
    height: 36px;
    border-radius: 8px;
    border: 2px dashed #555;
    background: transparent;
    color: #888;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: all 0.2s;
  }

  .add-project:hover {
    border-color: #f55247;
    color: #f55247;
  }

  .projects {
    display: flex;
    flex-direction: column;
    gap: 8px;
    margin-top: 8px;
  }

  .project-btn {
    width: 36px;
    height: 36px;
    border-radius: 8px;
    border: none;
    background: #333;
    color: #ccc;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.2s;
  }

  .project-btn:hover {
    background: #444;
  }

  .project-btn.active {
    background: #f55247;
    color: white;
  }

  .no-projects {
    writing-mode: vertical-rl;
    text-orientation: mixed;
    color: #555;
    font-size: 11px;
    margin-top: 16px;
    text-align: center;
  }
</style>
