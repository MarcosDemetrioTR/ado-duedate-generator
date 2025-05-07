package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/work"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
)

type WorkItem struct {
	ID      int        `json:"id"`
	Title   string     `json:"title"`
	Type    string     `json:"type"`
	State   string     `json:"state"`
	DueDate *time.Time `json:"dueDate"`
}

type Sprint struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	StartDate time.Time `json:"startDate,omitempty"`
	EndDate   time.Time `json:"endDate,omitempty"`
	IsCurrent bool      `json:"isCurrent"`
}

type Task struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	State       string `json:"state"`
	Description string `json:"description"`
	AssignedTo  string `json:"assignedTo"`
}

type DayOff struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

type TeamMemberCapacity struct {
	Activities []struct {
		CapacityPerDay float64 `json:"capacityPerDay"`
		Name           string  `json:"name"`
	} `json:"activities"`
	DaysOff []DayOff `json:"daysOff"`
}

type Developer struct {
	Name           string  `json:"name"`
	Email          string  `json:"email"`
	Tasks          int     `json:"tasks"`
	CapacityPerDay float64 `json:"capacityPerDay"`
	TotalCapacity  float64 `json:"totalCapacity"`
	DaysOff        int     `json:"daysOff"`
}

type DevelopersResponse struct {
	Developers    []Developer `json:"developers"`
	SprintStart   time.Time   `json:"sprintStart"`
	SprintEnd     time.Time   `json:"sprintEnd"`
	TotalCapacity float64     `json:"totalCapacity"`
	TotalDaysOff  int         `json:"totalDaysOff"`
	WorkingDays   int         `json:"workingDays"`
}

func getFieldValue(fields *map[string]interface{}, fieldName string) string {
	if fields == nil {
		return ""
	}
	if value, ok := (*fields)[fieldName]; ok {
		// Log para debug
		log.Printf("Campo %s encontrado com tipo %T e valor %v", fieldName, value, value)

		switch v := value.(type) {
		case string:
			return v
		case map[string]interface{}:
			// Para campos complexos, tenta obter o displayName ou value
			if displayName, ok := v["displayName"].(string); ok {
				return displayName
			}
			if val, ok := v["value"].(string); ok {
				return val
			}
		}
		// Se não conseguir converter, converte para string
		return fmt.Sprintf("%v", value)
	}
	return ""
}

// Middleware para adicionar headers CORS
func enableCors(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		handler(w, r)
	}
}

// Função para converter string de data para time.Time
func parseDate(dateStr string) (time.Time, error) {
	// Log para debug
	log.Printf("[DEBUG] Tentando converter data: %s", dateStr)

	// Tenta formatos conhecidos
	layouts := []string{
		"2006-01-02T15:04:05Z",      // ISO 8601 / RFC 3339
		"2006-01-02T15:04:05",       // ISO sem timezone
		"2006-01-02T15:04:05-07:00", // ISO com timezone
		"2006-01-02",                // Data simples
		"02/01/2006 15:04",          // BR com hora
		"02/01/2006",                // BR sem hora
		"1/2/2006",                  // Formato curto
		"January 2, 2006",           // Formato longo em inglês
		"2006/01/02",                // Formato com barras
	}

	for _, layout := range layouts {
		if t, err := time.Parse(layout, dateStr); err == nil {
			log.Printf("[DEBUG] Data convertida com sucesso usando layout: %s", layout)
			return t, nil
		}
	}

	// Se nenhum formato padrão funcionar, tenta parsear como RFC3339 ou ISO8601
	if t, err := time.Parse(time.RFC3339, dateStr); err == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("formato de data não reconhecido: %s", dateStr)
}

// Função para calcular dias úteis entre duas datas
func calculateWorkingDays(start, end time.Time, daysOff []DayOff) int {
	workingDays := 0
	current := start

	for current.Before(end) || current.Equal(end) {
		// Verifica se é fim de semana
		if current.Weekday() != time.Saturday && current.Weekday() != time.Sunday {
			// Verifica se é um dia de folga
			isDayOff := false
			for _, off := range daysOff {
				if (current.Equal(off.Start) || current.After(off.Start)) &&
					(current.Equal(off.End) || current.Before(off.End)) {
					isDayOff = true
					break
				}
			}
			if !isDayOff {
				workingDays++
			}
		}
		current = current.Add(24 * time.Hour)
	}

	return workingDays
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Erro ao carregar arquivo .env")
	}

	pat := os.Getenv("AZURE_DEVOPS_PAT")
	organization := os.Getenv("AZURE_DEVOPS_ORG")
	project := os.Getenv("AZURE_DEVOPS_PROJECT")
	team := os.Getenv("AZURE_DEVOPS_TEAM")

	if pat == "" || organization == "" || project == "" || team == "" {
		log.Fatal("Todas as variáveis de ambiente são obrigatórias: AZURE_DEVOPS_PAT, AZURE_DEVOPS_ORG, AZURE_DEVOPS_PROJECT, AZURE_DEVOPS_TEAM")
	}

	connection := azuredevops.NewPatConnection(organization, pat)

	// Endpoint para listar sprints
	http.HandleFunc("/sprints", enableCors(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.Background()
		workClient, err := work.NewClient(ctx, connection)
		if err != nil {
			http.Error(w, fmt.Sprintf("Erro ao criar cliente do Azure DevOps: %v", err), http.StatusInternalServerError)
			return
		}

		iterations, err := workClient.GetTeamIterations(ctx, work.GetTeamIterationsArgs{
			Project: &project,
			Team:    &team,
		})
		if err != nil {
			http.Error(w, fmt.Sprintf("Erro ao buscar sprints: %v", err), http.StatusInternalServerError)
			return
		}

		var allSprints []Sprint
		var currentSprintIndex int = -1
		now := time.Now()

		if iterations != nil && len(*iterations) > 0 {
			// Primeiro, vamos converter todas as iterações em sprints e identificar a atual
			for i, iteration := range *iterations {
				if iteration.Name == nil {
					continue
				}

				sprint := Sprint{
					Name: *iteration.Name,
				}

				if iteration.Path != nil {
					iterationID, err := uuid.Parse(*iteration.Path)
					if err == nil {
						sprint.ID = iterationID
					}
				}

				if iteration.Attributes != nil {
					if iteration.Attributes.StartDate != nil {
						sprint.StartDate = time.Time(iteration.Attributes.StartDate.Time)
					}
					if iteration.Attributes.FinishDate != nil {
						sprint.EndDate = time.Time(iteration.Attributes.FinishDate.Time)
					}

					// Verifica se é a sprint atual
					if !sprint.StartDate.IsZero() && !sprint.EndDate.IsZero() {
						if now.After(sprint.StartDate) && now.Before(sprint.EndDate) {
							sprint.IsCurrent = true
							currentSprintIndex = i
						}
					}
				}

				allSprints = append(allSprints, sprint)
			}

			// Se encontramos a sprint atual, vamos filtrar para mostrar apenas 3 antes e 3 depois
			var filteredSprints []Sprint
			if currentSprintIndex >= 0 {
				startIndex := currentSprintIndex - 3
				if startIndex < 0 {
					startIndex = 0
				}
				endIndex := currentSprintIndex + 4 // +4 porque o slice é exclusivo no final
				if endIndex > len(allSprints) {
					endIndex = len(allSprints)
				}
				filteredSprints = allSprints[startIndex:endIndex]
			} else {
				// Se não encontrou a sprint atual, retorna as últimas 7 sprints
				if len(allSprints) > 7 {
					filteredSprints = allSprints[len(allSprints)-7:]
				} else {
					filteredSprints = allSprints
				}
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(filteredSprints)
		} else {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]Sprint{})
		}
	}))

	// Função para retornar erro em formato JSON
	jsonError := func(w http.ResponseWriter, message string, code int) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		json.NewEncoder(w).Encode(map[string]string{"error": message})
	}

	http.HandleFunc("/user-stories", enableCors(func(w http.ResponseWriter, r *http.Request) {
		sprintName := r.URL.Query().Get("sprint")
		if sprintName == "" {
			jsonError(w, "Parâmetro 'sprint' é obrigatório", http.StatusBadRequest)
			return
		}

		ctx := context.Background()
		workClient, err := work.NewClient(ctx, connection)
		if err != nil {
			log.Printf("Erro ao criar cliente do Azure DevOps: %v", err)
			jsonError(w, fmt.Sprintf("Erro ao criar cliente do Azure DevOps: %v", err), http.StatusInternalServerError)
			return
		}

		// Buscar o ID da sprint pelo nome
		iterations, err := workClient.GetTeamIterations(ctx, work.GetTeamIterationsArgs{
			Project: &project,
			Team:    &team,
		})
		if err != nil {
			log.Printf("Erro ao buscar sprints: %v", err)
			jsonError(w, fmt.Sprintf("Erro ao buscar sprints: %v", err), http.StatusInternalServerError)
			return
		}

		var targetIteration *work.TeamSettingsIteration
		for _, iteration := range *iterations {
			if *iteration.Name == sprintName {
				targetIteration = &iteration
				break
			}
		}

		if targetIteration == nil {
			jsonError(w, fmt.Sprintf("Sprint '%s' não encontrada", sprintName), http.StatusNotFound)
			return
		}

		// Buscar work items da sprint
		workItemsResponse, err := workClient.GetIterationWorkItems(ctx, work.GetIterationWorkItemsArgs{
			Project:     &project,
			Team:        &team,
			IterationId: targetIteration.Id,
		})
		if err != nil {
			log.Printf("Erro ao buscar work items da sprint: %v", err)
			jsonError(w, fmt.Sprintf("Erro ao buscar work items: %v", err), http.StatusInternalServerError)
			return
		}

		// Criar cliente para buscar detalhes dos work items
		witClient, err := workitemtracking.NewClient(ctx, connection)
		if err != nil {
			log.Printf("Erro ao criar cliente de work items: %v", err)
			jsonError(w, fmt.Sprintf("Erro ao criar cliente de work items: %v", err), http.StatusInternalServerError)
			return
		}

		var workItemIds []int
		if workItemsResponse != nil && workItemsResponse.WorkItemRelations != nil {
			for _, relation := range *workItemsResponse.WorkItemRelations {
				if relation.Target != nil && relation.Target.Id != nil {
					workItemIds = append(workItemIds, *relation.Target.Id)
				}
			}
		}

		result := make([]WorkItem, 0)
		if len(workItemIds) > 0 {
			log.Printf("Buscando detalhes para %d work items", len(workItemIds))
			workItems, err := witClient.GetWorkItems(ctx, workitemtracking.GetWorkItemsArgs{
				Ids: &workItemIds,
				Fields: &[]string{
					"System.Title",
					"System.WorkItemType",
					"System.State",
					"Microsoft.VSTS.Scheduling.DueDate",
					"Microsoft.VSTS.Scheduling.TargetDate",
					"System.BoardColumn",
				},
				Project: &project,
			})

			if err != nil {
				log.Printf("Erro ao buscar detalhes dos work items: %v", err)
				jsonError(w, fmt.Sprintf("Erro ao buscar detalhes dos work items: %v", err), http.StatusInternalServerError)
				return
			}

			for _, detail := range *workItems {
				workItemType := getFieldValue(detail.Fields, "System.WorkItemType")
				if workItemType == "User Story" {
					log.Printf("Processando User Story #%d", *detail.Id)

					item := WorkItem{
						ID:      *detail.Id,
						Title:   getFieldValue(detail.Fields, "System.Title"),
						Type:    workItemType,
						State:   getFieldValue(detail.Fields, "System.State"),
						DueDate: nil,
					}

					// Log dos campos disponíveis
					log.Printf("=== Campos disponíveis para US #%d ===", *detail.Id)
					for fieldName, fieldValue := range *detail.Fields {
						log.Printf("[DEBUG] Campo %s = %v (tipo: %T)", fieldName, fieldValue, fieldValue)
					}

					// Tentar obter a data de diferentes campos
					dateFields := []string{
						"Microsoft.VSTS.Scheduling.DueDate",
						"Microsoft.VSTS.Scheduling.TargetDate",
						"Microsoft.VSTS.Common.DueDate",
					}

					var dueDateStr string
					for _, field := range dateFields {
						dueDateStr = getFieldValue(detail.Fields, field)
						if dueDateStr != "" {
							log.Printf("[DEBUG] Data encontrada no campo %s para US #%d: %s", field, *detail.Id, dueDateStr)
							break
						}
					}

					if dueDateStr != "" {
						log.Printf("[DEBUG] Tentando converter data '%s' para US #%d", dueDateStr, *detail.Id)
						if dueDate, err := parseDate(dueDateStr); err == nil {
							item.DueDate = &dueDate
							log.Printf("[DEBUG] Data convertida com sucesso para US #%d: %v", *detail.Id, dueDate)
						} else {
							log.Printf("[ERROR] Erro ao converter data '%s' para US #%d: %v", dueDateStr, *detail.Id, err)
						}
					} else {
						log.Printf("[DEBUG] Nenhuma data encontrada para US #%d nos campos: %v", *detail.Id, dateFields)
					}

					result = append(result, item)
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(result); err != nil {
			log.Printf("Erro ao codificar resposta JSON: %v", err)
			jsonError(w, "Erro ao processar resposta", http.StatusInternalServerError)
			return
		}
	}))

	http.HandleFunc("/user-story-tasks/", enableCors(func(w http.ResponseWriter, r *http.Request) {
		// Extrair ID da User Story da URL
		userStoryID := r.URL.Path[len("/user-story-tasks/"):]
		if userStoryID == "" {
			http.Error(w, "ID da User Story é obrigatório", http.StatusBadRequest)
			return
		}

		id, err := strconv.Atoi(userStoryID)
		if err != nil {
			http.Error(w, "ID da User Story inválido", http.StatusBadRequest)
			return
		}

		ctx := context.Background()
		witClient, err := workitemtracking.NewClient(ctx, connection)
		if err != nil {
			http.Error(w, fmt.Sprintf("Erro ao criar cliente do Azure DevOps: %v", err), http.StatusInternalServerError)
			return
		}

		// Buscar tasks vinculadas à User Story
		wiql := fmt.Sprintf(`SELECT [System.Id], [System.Title], [System.State], [System.Description], [System.AssignedTo] 
							FROM WorkItems 
							WHERE [System.WorkItemType] = 'Task' 
							AND [System.Parent] = %d`, id)

		query := workitemtracking.Wiql{Query: &wiql}
		queryResults, err := witClient.QueryByWiql(ctx, workitemtracking.QueryByWiqlArgs{
			Wiql:    &query,
			Project: &project,
		})

		if err != nil {
			http.Error(w, fmt.Sprintf("Erro ao buscar tasks: %v", err), http.StatusInternalServerError)
			return
		}

		var taskIds []int
		if queryResults != nil && queryResults.WorkItems != nil {
			for _, item := range *queryResults.WorkItems {
				if item.Id != nil {
					taskIds = append(taskIds, *item.Id)
				}
			}
		}

		tasks := make([]Task, 0)
		if len(taskIds) > 0 {
			workItems, err := witClient.GetWorkItems(ctx, workitemtracking.GetWorkItemsArgs{
				Ids:     &taskIds,
				Fields:  &[]string{"System.Title", "System.State", "System.Description", "System.AssignedTo"},
				Project: &project,
			})

			if err != nil {
				http.Error(w, fmt.Sprintf("Erro ao buscar detalhes das tasks: %v", err), http.StatusInternalServerError)
				return
			}

			for _, workItem := range *workItems {
				task := Task{
					ID:    *workItem.Id,
					Title: getFieldValue(workItem.Fields, "System.Title"),
					State: getFieldValue(workItem.Fields, "System.State"),
				}

				// Campos opcionais
				if desc := getFieldValue(workItem.Fields, "System.Description"); desc != "" {
					task.Description = desc
				}
				if assignedTo := getFieldValue(workItem.Fields, "System.AssignedTo"); assignedTo != "" {
					task.AssignedTo = assignedTo
				}

				tasks = append(tasks, task)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tasks)
	}))

	http.HandleFunc("/developers", enableCors(func(w http.ResponseWriter, r *http.Request) {
		sprintName := r.URL.Query().Get("sprint")
		if sprintName == "" {
			jsonError(w, "Parâmetro 'sprint' é obrigatório", http.StatusBadRequest)
			return
		}

		ctx := context.Background()
		workClient, err := work.NewClient(ctx, connection)
		if err != nil {
			jsonError(w, fmt.Sprintf("Erro ao criar cliente do Azure DevOps: %v", err), http.StatusInternalServerError)
			return
		}

		// Buscar o ID da sprint pelo nome
		iterations, err := workClient.GetTeamIterations(ctx, work.GetTeamIterationsArgs{
			Project: &project,
			Team:    &team,
		})
		if err != nil {
			jsonError(w, fmt.Sprintf("Erro ao buscar sprints: %v", err), http.StatusInternalServerError)
			return
		}

		var targetIteration *work.TeamSettingsIteration
		for _, iteration := range *iterations {
			if *iteration.Name == sprintName {
				targetIteration = &iteration
				break
			}
		}

		if targetIteration == nil {
			jsonError(w, fmt.Sprintf("Sprint '%s' não encontrada", sprintName), http.StatusNotFound)
			return
		}

		// Calcular capacidade total e dias úteis
		var sprintStart, sprintEnd time.Time
		if targetIteration.Attributes != nil {
			if targetIteration.Attributes.StartDate != nil {
				sprintStart = time.Time(targetIteration.Attributes.StartDate.Time)
			}
			if targetIteration.Attributes.FinishDate != nil {
				sprintEnd = time.Time(targetIteration.Attributes.FinishDate.Time)
			}
		}

		// Buscar work items da sprint
		workItemsResponse, err := workClient.GetIterationWorkItems(ctx, work.GetIterationWorkItemsArgs{
			Project:     &project,
			Team:        &team,
			IterationId: targetIteration.Id,
		})
		if err != nil {
			jsonError(w, fmt.Sprintf("Erro ao buscar work items da sprint: %v", err), http.StatusInternalServerError)
			return
		}

		witClient, err := workitemtracking.NewClient(ctx, connection)
		if err != nil {
			jsonError(w, fmt.Sprintf("Erro ao criar cliente de work items: %v", err), http.StatusInternalServerError)
			return
		}

		// Primeiro, vamos buscar todas as User Stories da sprint
		var workItemIds []int
		if workItemsResponse != nil && workItemsResponse.WorkItemRelations != nil {
			for _, relation := range *workItemsResponse.WorkItemRelations {
				if relation.Target != nil && relation.Target.Id != nil {
					workItemIds = append(workItemIds, *relation.Target.Id)
				}
			}
		}

		// Mapa para contar tasks por desenvolvedor
		devMap := make(map[string]*Developer)

		if len(workItemIds) > 0 {
			// Buscar as User Stories
			workItems, err := witClient.GetWorkItems(ctx, workitemtracking.GetWorkItemsArgs{
				Ids:     &workItemIds,
				Fields:  &[]string{"System.Id", "System.WorkItemType"},
				Project: &project,
			})

			if err != nil {
				jsonError(w, fmt.Sprintf("Erro ao buscar User Stories: %v", err), http.StatusInternalServerError)
				return
			}

			// WIQL para buscar tasks vinculadas às User Stories da sprint
			var userStoryIds []string
			for _, wi := range *workItems {
				if getFieldValue(wi.Fields, "System.WorkItemType") == "User Story" {
					userStoryIds = append(userStoryIds, fmt.Sprintf("%d", *wi.Id))
				}
			}

			if len(userStoryIds) > 0 {
				wiql := fmt.Sprintf(`SELECT [System.Id], [System.AssignedTo] 
								   FROM WorkItems 
								   WHERE [System.WorkItemType] = 'Task' 
								   AND [System.Parent] IN (%s)
								   AND [System.AssignedTo] <> ''`,
					strings.Join(userStoryIds, ","))

				query := workitemtracking.Wiql{Query: &wiql}
				queryResults, err := witClient.QueryByWiql(ctx, workitemtracking.QueryByWiqlArgs{
					Wiql:    &query,
					Project: &project,
				})

				if err != nil {
					jsonError(w, fmt.Sprintf("Erro ao buscar tasks: %v", err), http.StatusInternalServerError)
					return
				}

				var taskIds []int
				if queryResults != nil && queryResults.WorkItems != nil {
					for _, item := range *queryResults.WorkItems {
						if item.Id != nil {
							taskIds = append(taskIds, *item.Id)
						}
					}
				}

				if len(taskIds) > 0 {
					tasks, err := witClient.GetWorkItems(ctx, workitemtracking.GetWorkItemsArgs{
						Ids:     &taskIds,
						Fields:  &[]string{"System.AssignedTo"},
						Project: &project,
					})

					if err != nil {
						jsonError(w, fmt.Sprintf("Erro ao buscar detalhes das tasks: %v", err), http.StatusInternalServerError)
						return
					}

					for _, task := range *tasks {
						if assignedTo := getFieldValue(task.Fields, "System.AssignedTo"); assignedTo != "" {
							if dev, exists := devMap[assignedTo]; exists {
								dev.Tasks++
							} else {
								devMap[assignedTo] = &Developer{
									Name:  assignedTo,
									Tasks: 1,
								}
							}
						}
					}
				}
			}
		}

		// Mapa para armazenar capacidade por desenvolvedor
		devCapacities := make(map[string]TeamMemberCapacity)

		// Definir capacidade padrão para todos os desenvolvedores
		for _, dev := range devMap {
			devCapacities[dev.Name] = TeamMemberCapacity{
				Activities: []struct {
					CapacityPerDay float64 `json:"capacityPerDay"`
					Name           string  `json:"name"`
				}{
					{
						CapacityPerDay: 8.0, // 8 horas por dia como padrão
						Name:           "Desenvolvimento",
					},
				},
				DaysOff: []DayOff{},
			}
		}

		response := DevelopersResponse{
			SprintStart: sprintStart,
			SprintEnd:   sprintEnd,
		}

		// Converter mapa para slice e calcular capacidades
		developers := make([]Developer, 0, len(devMap))
		totalDaysOff := 0
		for _, dev := range devMap {
			developer := Developer{
				Name:  dev.Name,
				Tasks: dev.Tasks,
			}

			if capacity, exists := devCapacities[dev.Name]; exists {
				// Soma todas as capacidades por dia
				for _, activity := range capacity.Activities {
					developer.CapacityPerDay += activity.CapacityPerDay
				}

				// Calcula dias úteis considerando dias de folga
				workingDays := calculateWorkingDays(sprintStart, sprintEnd, capacity.DaysOff)
				developer.DaysOff = len(capacity.DaysOff)
				totalDaysOff += developer.DaysOff

				// Calcula capacidade total
				developer.TotalCapacity = float64(workingDays) * developer.CapacityPerDay
				response.TotalCapacity += developer.TotalCapacity
			}

			developers = append(developers, developer)
		}

		// Ordenar por nome
		sort.Slice(developers, func(i, j int) bool {
			return developers[i].Name < developers[j].Name
		})

		response.Developers = developers
		response.TotalDaysOff = totalDaysOff
		response.WorkingDays = calculateWorkingDays(sprintStart, sprintEnd, nil)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))

	port := ":8088"
	fmt.Printf("Servidor rodando na porta %s\n", port)
	log.Fatal(http.ListenAndServe(port, nil))
}
