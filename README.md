# Azure DevOps Sprint User Stories API

API em Go para listar User Stories de uma Sprint específica no Azure DevOps.

## Pré-requisitos

- Go 1.21 ou superior
- Personal Access Token (PAT) do Azure DevOps com as seguintes permissões:
  - Work Items (Read)
  - Project and Team (Read)

## Configuração

1. Instale o Go em seu sistema:
   - Faça o download do instalador em: https://golang.org/dl/
   - Siga as instruções de instalação para Windows

2. Clone este repositório:
   ```powershell
   git clone <seu-repositorio>
   cd azuredevops
   ```

3. Configure as variáveis de ambiente:
   - Crie um arquivo `.env` na raiz do projeto
   - Adicione as seguintes variáveis:
     ```
     AZURE_DEVOPS_PAT=seu_pat_aqui
     AZURE_DEVOPS_ORG=https://dev.azure.com/sua_organizacao
     AZURE_DEVOPS_PROJECT=seu_projeto
     AZURE_DEVOPS_TEAM=nome_do_seu_time
     ```

4. Instale as dependências:
   ```powershell
   go mod download
   ```

## Dependências

O projeto utiliza as seguintes bibliotecas:
- `github.com/joho/godotenv v1.5.1` - Para carregar variáveis de ambiente do arquivo .env
- `github.com/microsoft/azure-devops-go-api/azuredevops/v7 v7.1.0` - SDK oficial do Azure DevOps

## Executando o projeto

1. Inicie o servidor:
   ```powershell
   go run main.go
   ```

2. O servidor estará disponível em `http://localhost:8080`

## Uso da API

### Listar User Stories de uma Sprint

```http
GET /user-stories?sprint=nome_da_sprint
```

#### Parâmetros da Query
- `sprint` (obrigatório): Nome exato da sprint no Azure DevOps

#### Exemplo de resposta
```json
[
  {
    "id": 123,
    "title": "Como usuário, eu quero...",
    "type": "User Story",
    "state": "Active"
  }
]
```

#### Códigos de Resposta
- `200 OK` - Sucesso
- `400 Bad Request` - Sprint não informada
- `404 Not Found` - Sprint não encontrada
- `500 Internal Server Error` - Erro ao comunicar com Azure DevOps

## Funcionalidades

- [x] Autenticação via PAT
- [x] Busca de sprints por nome
- [x] Listagem de User Stories da sprint
- [x] Filtragem apenas de itens do tipo "User Story"
- [x] Tratamento de erros robusto
- [x] Otimização de chamadas à API (busca em lote)
- [x] Validação de variáveis de ambiente

## Observações Importantes

1. O PAT deve ter permissões adequadas para leitura de Work Items
2. O nome da sprint na URL deve ser exatamente igual ao configurado no Azure DevOps
3. O nome do time deve ser exatamente igual ao configurado no Azure DevOps
4. A API retorna apenas itens do tipo "User Story"
5. Os campos retornados são:
   - ID
   - Título
   - Tipo (sempre "User Story")
   - Estado

## Tratamento de Erros

A API inclui tratamento robusto de erros, incluindo:
- Validação de variáveis de ambiente obrigatórias
- Verificação de parâmetros da requisição
- Tratamento de erros da API do Azure DevOps
- Mensagens de erro descritivas

## Segurança

- Não expõe informações sensíveis nas mensagens de erro
- Utiliza HTTPS para comunicação com Azure DevOps
- Validação de todas as entradas do usuário
- Não armazena credenciais em código

## Suporte

Para reportar problemas ou sugerir melhorias, abra uma issue no repositório. 