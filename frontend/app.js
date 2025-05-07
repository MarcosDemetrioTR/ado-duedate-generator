// Configuração da API
const API_URL = 'http://localhost:8088';

// Elementos do DOM
const sprintSelect = document.getElementById('sprint-select');
const stateSelect = document.getElementById('state-select');
const storiesContainer = document.getElementById('stories-container');
const loadingSpinner = document.getElementById('loading-spinner');

// Cache de histórias e tasks
let userStoriesCache = [];
let tasksCache = new Map();
let expandedStories = new Set(); // Guarda o estado de expansão das USs

// Função para mostrar/ocultar o spinner de carregamento
function toggleLoading(show) {
    if (loadingSpinner) loadingSpinner.style.display = show ? 'block' : 'none';
    if (storiesContainer) storiesContainer.style.display = show ? 'none' : 'block';
}

// Função para formatar o estado com badge
function getStateBadge(state) {
    const stateColors = {
        'New': 'primary',
        'Active': 'info',
        'Resolved': 'warning',
        'Closed': 'success',
        'Removed': 'danger'
    };
    const colorClass = stateColors[state] || 'secondary';
    return `<span class="badge bg-${colorClass}">${state}</span>`;
}

// Função para formatar data
function formatDate(dateString) {
    if (!dateString) return '';
    const date = new Date(dateString);
    if (isNaN(date.getTime())) return '';
    
    return date.toLocaleDateString('pt-BR', {
        day: '2-digit',
        month: '2-digit',
        year: 'numeric'
    });
}

// Função para formatar o DueDate com badge
function getDueDateBadge(dueDate) {
    // Se dueDate for null, undefined ou string vazia
    if (!dueDate || dueDate === "") {
        return '<span class="badge bg-orange text-dark"><i class="bi bi-calendar-x"></i> Não informado</span>';
    }
    
    const date = new Date(dueDate);
    // Verifica se a data é válida
    if (isNaN(date.getTime())) {
        return '<span class="badge bg-orange text-dark"><i class="bi bi-calendar-x"></i> Não informado</span>';
    }
    
    const today = new Date();
    // Reseta as horas para comparar apenas as datas
    today.setHours(0, 0, 0, 0);
    const compareDate = new Date(date);
    compareDate.setHours(0, 0, 0, 0);
    
    const diffDays = Math.ceil((compareDate - today) / (1000 * 60 * 60 * 24));
    
    let colorClass = 'secondary';
    let textClass = '';
    if (diffDays < 0) {
        colorClass = 'danger';  // Atrasado
    } else if (diffDays <= 7) {
        colorClass = 'warning';  // Próximo da data
        textClass = 'text-dark'; // Texto escuro para melhor contraste
    } else {
        colorClass = 'info';  // Dentro do prazo
        textClass = 'text-dark'; // Texto escuro para melhor contraste
    }
    
    return `<span class="badge bg-${colorClass} ${textClass}"><i class="bi bi-calendar"></i> ${formatDate(dueDate)}</span>`;
}

// Função para renderizar tasks de uma User Story
function renderTasks(userStoryId) {
    const tasks = tasksCache.get(userStoryId) || [];
    if (tasks.length === 0) {
        return '<div class="alert alert-info mt-3">Nenhuma task encontrada.</div>';
    }

    return tasks.map(task => `
        <div class="card task-card mt-2">
            <div class="card-body">
                <div class="d-flex justify-content-between align-items-start">
                    <div>
                        <h6 class="card-subtitle mb-2">#${task.id} - ${task.title}</h6>
                        ${getStateBadge(task.state)}
                    </div>
                    ${task.assignedTo ? `<small class="text-muted">Atribuído para: ${task.assignedTo}</small>` : ''}
                </div>
                ${task.description ? `<p class="card-text mt-2">${task.description}</p>` : ''}
            </div>
        </div>
    `).join('');
}

// Função para mostrar mensagem de erro
function showError(message) {
    if (storiesContainer) {
        storiesContainer.innerHTML = `
            <div class="alert alert-danger">
                <i class="bi bi-exclamation-triangle-fill me-2"></i>
                ${message}
            </div>
        `;
    }
}

// Função para carregar tasks de uma User Story
async function loadTasks(userStoryId) {
    try {
        const response = await fetch(`${API_URL}/user-story-tasks/${userStoryId}`);
        const data = await response.json();
        
        if (!response.ok) {
            throw new Error(data.error || 'Erro ao carregar tasks');
        }
        
        tasksCache.set(userStoryId, data);
        return data;
    } catch (error) {
        console.error('Erro ao carregar tasks:', error);
        return [];
    }
}

// Função para alternar a visibilidade das tasks
async function toggleTasks(userStoryId) {
    const tasksContainer = document.getElementById(`tasks-${userStoryId}`);
    const toggleButton = document.getElementById(`toggle-${userStoryId}`);
    
    if (!tasksContainer) return;

    if (tasksContainer.style.display === 'none' || !tasksContainer.innerHTML.trim()) {
        // Se as tasks ainda não foram carregadas
        if (!tasksCache.has(userStoryId)) {
            toggleButton.innerHTML = '<span class="spinner-border spinner-border-sm" role="status"></span>';
            await loadTasks(userStoryId);
            toggleButton.innerHTML = '<i class="bi bi-chevron-up"></i>';
        }
        
        tasksContainer.innerHTML = renderTasks(userStoryId);
        tasksContainer.style.display = 'block';
        toggleButton.querySelector('i').classList.replace('bi-chevron-down', 'bi-chevron-up');
        expandedStories.add(userStoryId);
    } else {
        tasksContainer.style.display = 'none';
        toggleButton.querySelector('i').classList.replace('bi-chevron-up', 'bi-chevron-down');
        expandedStories.delete(userStoryId);
    }
}

// Função para carregar as sprints
async function loadSprints() {
    try {
        const response = await fetch(`${API_URL}/sprints`);
        const data = await response.json();
        
        if (!response.ok) {
            throw new Error(data.error || 'Erro ao carregar sprints');
        }
        
        // Limpa as opções existentes
        sprintSelect.innerHTML = '<option value="">Selecione uma Sprint</option>';
        
        // Adiciona as novas opções
        data.forEach(sprint => {
            const option = document.createElement('option');
            option.value = sprint.name;
            option.textContent = sprint.isCurrent ? `${sprint.name} (Sprint Atual)` : sprint.name;
            if (sprint.isCurrent) {
                option.classList.add('fw-bold');
                option.selected = true;
            }
            sprintSelect.appendChild(option);
        });

        // Se uma sprint foi selecionada, carrega as histórias
        if (sprintSelect.value) {
            await loadUserStories(sprintSelect.value);
        }
    } catch (error) {
        console.error('Erro ao carregar sprints:', error);
        if (sprintSelect) {
            sprintSelect.innerHTML = `
                <option value="">Erro ao carregar sprints: ${error.message}</option>
            `;
        }
        showError(`Erro ao carregar sprints: ${error.message}`);
    }
}

// Função para carregar as histórias de usuário
async function loadUserStories(sprintName) {
    toggleLoading(true);
    try {
        const response = await fetch(`${API_URL}/user-stories?sprint=${encodeURIComponent(sprintName)}`);
        const data = await response.json();
        
        if (!response.ok) {
            throw new Error(data.error || 'Erro ao carregar histórias');
        }
        
        userStoriesCache = data;
        tasksCache.clear();
        expandedStories.clear();
        filterAndDisplayStories();
    } catch (error) {
        console.error('Erro ao carregar histórias:', error);
        showError(`Erro ao carregar histórias: ${error.message}`);
        userStoriesCache = [];
    } finally {
        toggleLoading(false);
    }
}

// Função para filtrar e exibir as histórias
function filterAndDisplayStories() {
    if (!storiesContainer) return;

    const selectedState = stateSelect.value;
    const filteredStories = selectedState
        ? userStoriesCache.filter(story => story.state === selectedState)
        : userStoriesCache;

    if (filteredStories.length === 0) {
        storiesContainer.innerHTML = '<div class="alert alert-info">Nenhuma história encontrada.</div>';
        return;
    }

    const storiesHTML = filteredStories.map(story => `
        <div class="card mb-3 story-card">
            <div class="card-body">
                <div class="d-flex justify-content-between align-items-start mb-3">
                    <h5 class="card-title mb-0">#${story.id} - ${story.title}</h5>
                    <div class="d-flex align-items-center gap-2">
                        ${getDueDateBadge(story.dueDate)}
                        ${getStateBadge(story.state)}
                        <button class="btn btn-link" id="toggle-${story.id}" onclick="toggleTasks(${story.id})">
                            <i class="bi bi-chevron-${expandedStories.has(story.id) ? 'up' : 'down'}"></i>
                        </button>
                    </div>
                </div>
                <div id="tasks-${story.id}" class="tasks-container" style="display: ${expandedStories.has(story.id) ? 'block' : 'none'}">
                    ${expandedStories.has(story.id) ? renderTasks(story.id) : ''}
                </div>
            </div>
        </div>
    `).join('');

    storiesContainer.innerHTML = storiesHTML;
}

// Função para formatar número com 1 casa decimal
function formatNumber(num) {
    return Number(num).toFixed(1);
}

// Função para carregar desenvolvedores
async function loadDevelopers() {
    const devsList = document.getElementById('devs-list');
    if (!devsList) return;

    const sprintSelect = document.getElementById('sprint-select');
    const selectedSprint = sprintSelect ? sprintSelect.value : '';

    if (!selectedSprint) {
        devsList.innerHTML = `
            <div class="alert alert-info">
                Selecione uma sprint para ver os desenvolvedores.
            </div>
        `;
        return;
    }

    try {
        devsList.innerHTML = `
            <div class="text-center py-3">
                <div class="spinner-border text-primary" role="status">
                    <span class="visually-hidden">Carregando...</span>
                </div>
            </div>
        `;

        const response = await fetch(`${API_URL}/developers?sprint=${encodeURIComponent(selectedSprint)}`);
        const data = await response.json();

        if (!response.ok) {
            throw new Error(data.error || 'Erro ao carregar desenvolvedores');
        }

        if (!data.developers || data.developers.length === 0) {
            devsList.innerHTML = `
                <div class="alert alert-info">
                    Nenhum desenvolvedor encontrado com tasks atribuídas nesta sprint.
                </div>
            `;
            return;
        }

        // Calcula o período da sprint
        const sprintStart = new Date(data.sprintStart);
        const sprintEnd = new Date(data.sprintEnd);
        const sprintPeriod = `${sprintStart.toLocaleDateString('pt-BR')} - ${sprintEnd.toLocaleDateString('pt-BR')}`;

        const devsHTML = data.developers.map(dev => `
            <div class="list-group-item">
                <div class="d-flex justify-content-between align-items-center mb-2">
                    <div>
                        <i class="bi bi-person-circle me-2"></i>
                        ${dev.name}
                    </div>
                    <span class="badge bg-primary rounded-pill">
                        ${dev.tasks} task${dev.tasks === 1 ? '' : 's'}
                    </span>
                </div>
                <div class="capacity-info small">
                    <div class="d-flex justify-content-between text-muted">
                        <span>Capacidade por dia:</span>
                        <span>${formatNumber(dev.capacityPerDay)}h</span>
                    </div>
                    <div class="d-flex justify-content-between text-muted">
                        <span>Dias de folga:</span>
                        <span>${dev.daysOff} dia${dev.daysOff === 1 ? '' : 's'}</span>
                    </div>
                    <div class="d-flex justify-content-between">
                        <span class="fw-bold">Capacidade total:</span>
                        <span class="fw-bold">${formatNumber(dev.totalCapacity)}h</span>
                    </div>
                </div>
            </div>
        `).join('');

        // Adiciona o resumo da sprint
        const summaryHTML = `
            <div class="card mt-4">
                <div class="card-body">
                    <h6 class="card-subtitle mb-3 text-muted">
                        Período da Sprint: ${sprintPeriod}
                    </h6>
                    <div class="d-flex justify-content-between align-items-center mb-2">
                        <span>Dias úteis na sprint:</span>
                        <span class="fw-bold">${data.workingDays} dias</span>
                    </div>
                    <div class="d-flex justify-content-between align-items-center mb-2">
                        <span>Total de dias de folga:</span>
                        <span class="fw-bold">${data.totalDaysOff} dias</span>
                    </div>
                    <div class="d-flex justify-content-between align-items-center">
                        <span>Capacidade total da equipe:</span>
                        <span class="fw-bold">${formatNumber(data.totalCapacity)}h</span>
                    </div>
                </div>
            </div>
        `;

        devsList.innerHTML = devsHTML + summaryHTML;
    } catch (error) {
        console.error('Erro ao carregar desenvolvedores:', error);
        devsList.innerHTML = `
            <div class="alert alert-danger">
                <i class="bi bi-exclamation-triangle-fill me-2"></i>
                Erro ao carregar desenvolvedores: ${error.message}
            </div>
        `;
    }
}

// Event listeners
if (sprintSelect) {
    sprintSelect.addEventListener('change', async (e) => {
        if (e.target.value) {
            await loadUserStories(e.target.value);
            // Se estiver na aba de desenvolvedores, recarrega a lista
            const devsTab = document.getElementById('devs-content');
            if (devsTab && devsTab.classList.contains('active')) {
                await loadDevelopers();
            }
        } else {
            if (storiesContainer) storiesContainer.innerHTML = '';
            userStoriesCache = [];
            tasksCache.clear();
            expandedStories.clear();
            // Limpa a lista de desenvolvedores se estiver visível
            const devsList = document.getElementById('devs-list');
            if (devsList) {
                devsList.innerHTML = `
                    <div class="alert alert-info">
                        Selecione uma sprint para ver os desenvolvedores.
                    </div>
                `;
            }
        }
    });
}

if (stateSelect) {
    stateSelect.addEventListener('change', () => {
        filterAndDisplayStories();
    });
}

// Event listeners para as abas
document.addEventListener('DOMContentLoaded', () => {
    // Esconde o loading inicialmente
    toggleLoading(false);
    
    // Carrega as sprints
    loadSprints();

    // Adiciona listener para mudança de aba
    const devsTab = document.getElementById('devs-tab');
    if (devsTab) {
        devsTab.addEventListener('shown.bs.tab', () => {
            loadDevelopers();
        });
    }
}); 