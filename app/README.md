# MagicStrike 🚀

MagicStrike é um pipeline inteligente de processamento e análise tática de replays de CS2 (Counter-Strike 2). Ele faz o parse de arquivos `.dem` extraindo eventos estruturados, gera narrativas por round via IA (DeepSeek) e armazena os vetores (embeddings Voyage AI) no banco vetorial Qdrant para busca semântica e contextualização.

---

## 🛠️ Gerenciando a Infraestrutura de Desenvolvimento

Toda a infraestrutura é executada em contêineres locais. Você pode subir as dependências usando os seguintes utilitários de console:

* **Iniciar serviços locais**: `make infra-up`
* **Parar serviços locais**: `make infra-down`

### Painéis de Administração e Credenciais (Desenvolvimento)

| Serviço | Interface Web / Porta | Usuário | Senha | Banco Padrão |
| :--- | :--- | :--- | :--- | :--- |
| **PostgreSQL** | `localhost:5432` | `postgres` | `postgres` | `magicstrike` |
| **RabbitMQ** | [http://localhost:15672](http://localhost:15672) | `guest` | `guest` | `-` |
| **MinIO (S3)** | [http://localhost:9001](http://localhost:9001) | `minioadmin` | `minioadmin` | `magicstrike-demos` |
| **ClickHouse** | [http://localhost:8123/play](http://localhost:8123/play) | `default` | `test` | `magicstrike` |
| **Qdrant** | [http://127.0.0.1:6333/dashboard](http://localhost:6333/dashboard) | `-` *(Sem Auth)* | `-` *(Sem Auth)* | `round_narratives` |

---

## 🏗️ Arquitetura

O projeto adota os princípios da **Arquitetura Hexagonal (Ports & Adapters)** dividindo-se em:
* `internal/core/entities`: Regras de negócio puras e entidades autocontidas.
* `internal/core/ports`: Interfaces de entrada e saída.
* `internal/core/services` & `usecases`: Lógica de aplicação e orquestração de fluxos.
* `internal/adapters/in`: HTTP Handlers e Consumidores de mensageria.
* `internal/adapters/out`: Drivers de banco de dados e conexões com APIs (Postgres, ClickHouse, Qdrant, MinIO, RabbitMQ, Voyage, DeepSeek).

---

## 🚀 Como Rodar o Projeto

1. **Subir a Infraestrutura**:
   Inicie as dependências locais utilizando o Podman:
   ```bash
   make infra-up
   ```

2. **Configurar as Variáveis de Ambiente**:
   Copie o arquivo de exemplo de ambiente e ajuste as credenciais se necessário (por padrão já vem configurado para a infraestrutura local):
   ```bash
   cp .env.example .env
   ```
   Caso possua chaves para os serviços de IA, preencha `DEEPSEEK_API_KEY` e `VOYAGE_API_KEY` no arquivo `.env`.

3. **Compilar os Binários**:
   ```bash
   make build
   ```

4. **Rodar os Testes**:
   ```bash
   make test
   ```

5. **Executar as Aplicações**:
   * **API Principal (HTTP)**:
     ```bash
     make run-api
     ```
   * **Worker de Processamento de Demos (Modo CLI - Único Arquivo)**:
     ```bash
     ./bin/worker -mode cli -demo caminho/para/demo.dem
     ```
   * **Worker de Processamento de Demos (Modo Consumer - RabbitMQ)**:
     ```bash
     ./bin/worker -mode consumer
     ```

6. **Testar Requisições**:
   Utilize o arquivo utilitário **[api.http](file:///home/joaomigguel/Documentos/magicstrike/app/api.http)** na raiz do projeto com a extensão VS Code **REST Client** para disparar chamadas de autenticação, upload e chats de teste.