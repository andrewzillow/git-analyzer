# git-analyzer

Hack Day 2025

A command-line tool for analyzing GitHub and GitLab repositories.

## Features

- Authenticate with GitHub or GitLab using personal access tokens
- List and select repositories from your account
- Interactive command-line interface
- HTTP server with JSON API endpoints
- Support for git-blame and git-log analysis
- Token caching for improved user experience

## Prerequisites

- Go 1.24 or later
- GitHub or GitLab personal access token

## Installation

1. Clone the repository:
```bash
git clone https://github.com/yourusername/repo-analyzer.git
cd repo-analyzer
```

2. Build the tool:
```bash
go build -o repo-analyzer cmd/cli/main.go
```

## Usage

### Command Line Interface

Run the tool without any flags to start the interactive mode:

```bash
./repo-analyzer
```

### Using Flags

You can also provide the provider and token as flags:

```bash
./repo-analyzer --provider github --token your-token
```

### HTTP Server

Start the HTTP server:

```bash
./repo-analyzer server
```

By default, the server runs on port 8080. You can specify a different port:

```bash
./repo-analyzer server --port 3000
```

### API Endpoints

#### POST /messages

Accepts JSON requests with the following format:

```json
{
  "name": "git-blame" | "git-log",
  "arguments": {
    "provider": "github" | "gitlab",
    "token": "your-token",
    "repository": "owner/repo",
    "pullRequest": 1
  }
}
```

Example using curl:
```bash
curl -X POST -H "Content-Type: application/json" -d '{
  "name": "git-blame",
  "arguments": {
    "provider": "github",
    "token": "your-token",
    "repository": "owner/repo",
    "pullRequest": 1
  }
}' http://localhost:8080/messages
```

### Environment Variables

You can set your tokens as environment variables:

```bash
export GITHUB_TOKEN=your-github-token
export GITLAB_TOKEN=your-gitlab-token
```

## Message Types

### git-blame
Analyzes the blame information for files in a pull request, showing which authors modified which lines.

### git-log
Returns a success response for the specified repository and pull request.

## Getting a Personal Access Token

### GitHub
1. Go to GitHub Settings > Developer Settings > Personal Access Tokens
2. Generate a new token with the `repo` scope

### GitLab
1. Go to GitLab Settings > Access Tokens
2. Generate a new token with the `read_api` scope

## License

MIT 
