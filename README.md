# Go Code Runner

A backend service for running code execution and managing coding tests for technical interviews.

## Total time spent
I worked in total 20hrs on this project. Due to my full time job I was not able to spend much time.
I agree that there is gap for refactoring the code even further.

## Overview

Go Code Runner is a platform that allows companies to create coding tests for candidates and for candidates to take these tests. The service provides:

- Code execution in Go
- Problem management
- Company registration and authentication
- Coding test generation and management
- API for frontend integration

## Features

- **Code Execution**: Execute Go code snippets with or without test cases
- **Problem Management**: Create, retrieve, and list coding problems
- **Company Management**: Register, login, generate API keys and client IDs
- **Coding Test Management**: Generate, verify, start, and submit coding tests
- **Authentication**: JWT-based authentication for companies and API key authentication for test generation

## Prerequisites

- Go 1.16 or higher
- PostgreSQL
- Docker and Docker Compose (optional, for containerized deployment)

## Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/yourusername/go-code-runner.git
   cd go-code-runner
   ```

2. Install dependencies:
   ```bash
   go mod download
   ```

3. Set up environment variables:
   Create a `.env` file in the root directory with the following variables:
   ```
   DB_CONN_STR=postgres://username:password@localhost:5432/go_code_runner?sslmode=disable
   SERVER_PORT=8080
   JWT_SECRET=your_jwt_secret
   EXECUTION_TIMEOUT=10s
   ```

## Running the Application

### Using Go

```bash
make run
```

This will start the server on the port specified in your environment variables (default: 8080). If you have `air` installed, it will use it for live reloading.

### Using Docker

```bash
make docker-run
```

This will build and start the application using Docker Compose.

To stop the Docker containers:

```bash
make docker-stop
```

## Testing

### Running Unit Tests

```bash
make test
```

### HTTP Tests

The project includes HTTP test files in the `/tests/http_tests` directory. These files can be used with tools like [REST Client](https://marketplace.visualstudio.com/items?itemName=humao.rest-client) for VS Code or [HTTP Client](https://www.jetbrains.com/help/idea/http-client-in-product-code-editor.html) for JetBrains IDEs.

#### Available HTTP Test Files:

1. **auth_api_test.http**: Tests for authentication endpoints
2. **code_run_api.http**: Tests for code execution endpoints
3. **company_test.http**: Tests for company management endpoints
4. **generate_test_for_interviewee.http**: End-to-end flow for generating and taking a coding test
5. **problems_api.http**: Tests for problem management endpoints

#### Running HTTP Tests:

1. Ensure the server is running locally on port 8080
2. Open the HTTP test file in your IDE
3. Use the IDE's HTTP client to send the requests

Example flow using `generate_test_for_interviewee.http`:

1. Login with company credentials
2. Generate a client ID
3. Generate an API key
4. Generate a coding test with a specific problem
5. Verify the test
6. Start the test as a candidate
7. Execute code for the test
8. Submit the test with the passed percentage

## API Endpoints

### Health Check
- `GET /health`: Check if the server is running

### Code Execution
- `POST /api/v1/execute`: Execute code with optional problem ID

### Problem Management
- `GET /api/v1/problems`: List all problems
- `GET /api/v1/problems/:id`: Get a problem by ID

### Company Management
- `POST /api/v1/companies/register`: Register a new company
- `POST /api/v1/companies/login`: Login with company credentials
- `POST /api/v1/companies/api-key`: Generate an API key (requires JWT authentication)
- `POST /api/v1/companies/client-id`: Generate a client ID (requires JWT authentication)
- `GET /api/v1/companies/tests`: Get all tests for a company (requires JWT authentication)
- `POST /api/v1/companies/tests/generate`: Generate a new test (requires API key authentication)

### Coding Test Management
- `GET /api/v1/tests/:test_id/verify`: Verify a test
- `POST /api/v1/tests/:test_id/start`: Start a test
- `POST /api/v1/tests/:test_id/submit`: Submit a test

## Project Structure

- `cmd/server`: Entry point for the application
- `internal/server`: Server initialization and routing
- `internal/handler`: HTTP handlers
- `internal/service`: Business logic
- `internal/repository`: Data access
- `internal/models`: Data models
- `internal/middleware`: HTTP middleware
- `internal/code_executor`: Code execution logic
- `internal/config`: Configuration loading
- `internal/platform`: Infrastructure components
- `tests`: Test files

## License

[MIT License](LICENSE)