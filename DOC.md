# Documentação do Projeto Azure DevOps Sprint User Stories API

## Visão Geral
Este projeto é uma API desenvolvida em Go que interage com o Azure DevOps para gerenciar e consultar informações sobre User Stories, Sprints e capacidade da equipe. A API fornece endpoints para listar sprints, user stories, e calcular a capacidade dos desenvolvedores.

## Estrutura do Projeto
```
.
├── .git/
├── .idea/
├── frontend/
├── .gitignore
├── main.go
├── go.mod
├── go.sum
└── README.md
```

## Tecnologias Utilizadas
- Go 1.21+
- Azure DevOps Go API SDK
- godotenv para gerenciamento de variáveis de ambiente

## Dependências Principais
- github.com/joho/godotenv v1.5.1
- github.com/microsoft/azure-devops-go-api/azuredevops/v7 v7.1.0
- github.com/google/uuid

## Funcionalidades Implementadas

### 1. Gestão de Sprints
- Listagem de todas as sprints do time
- Identificação automática da sprint atual
- Informações detalhadas incluindo datas de início e fim

### 2. User Stories
- Listagem de User Stories por sprint
- Filtragem por tipo de item
- Informações detalhadas incluindo estado e data de vencimento

### 3. Gestão de Capacidade
- Cálculo de capacidade por desenvolvedor
- Consideração de dias de folga
- Cálculo de dias úteis entre datas
- Suporte a múltiplas atividades

### 4. Endpoints da API

#### GET /sprints
- Lista todas as sprints do time
- Retorna informações detalhadas incluindo datas e status

#### GET /user-stories
- Lista User Stories de uma sprint específica
- Parâmetros:
  - sprint: nome da sprint (obrigatório)

#### GET /developers
- Retorna informações sobre a capacidade dos desenvolvedores
- Inclui:
  - Nome e email
  - Número de tasks
  - Capacidade diária
  - Dias de folga
  - Capacidade total

## Estruturas de Dados

### WorkItem
```go
type WorkItem struct {
    ID      int
    Title   string
    Type    string
    State   string
    DueDate *time.Time
}
```

### Sprint
```go
type Sprint struct {
    ID        uuid.UUID
    Name      string
    StartDate time.Time
    EndDate   time.Time
    IsCurrent bool
}
```

### Developer
```go
type Developer struct {
    Name           string
    Email          string
    Tasks          int
    CapacityPerDay float64
    TotalCapacity  float64
    DaysOff        int
}
```

## Configuração do Ambiente

### Variáveis de Ambiente Necessárias
```
AZURE_DEVOPS_PAT=seu_pat_aqui
AZURE_DEVOPS_ORG=https://dev.azure.com/sua_organizacao
AZURE_DEVOPS_PROJECT=seu_projeto
AZURE_DEVOPS_TEAM=nome_do_seu_time
```

### Permissões do PAT
- Work Items (Read)
- Project and Team (Read)

## Segurança
- Autenticação via PAT
- Validação de todas as entradas
- Headers CORS configurados
- Tratamento seguro de erros

## Tratamento de Erros
- Validação de variáveis de ambiente
- Verificação de parâmetros de requisição
- Tratamento de erros da API do Azure DevOps
- Mensagens de erro descritivas

## Observações Importantes
1. O PAT deve ter permissões adequadas
2. Nomes de sprint e time devem corresponder exatamente ao Azure DevOps
3. A API retorna apenas itens do tipo "User Story"
4. Suporte a múltiplos formatos de data
5. Cálculo preciso de dias úteis considerando folgas

## Próximos Passos Sugeridos
1. Implementação de cache para otimizar performance
2. Adição de testes automatizados
3. Documentação da API com Swagger
4. Implementação de rate limiting
5. Adição de métricas de monitoramento 