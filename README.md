# git-analyzer

Hack Day 2025

A command-line tool for analyzing GitHub and GitLab repositories.

## Features

- Authenticate with GitHub or GitLab using personal access tokens
- List and select repositories from your account
- Interactive command-line interface

## Prerequisites

- Go 1.16 or later
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

### Basic Usage

Run the tool without any flags to start the interactive mode:

```bash
./repo-analyzer
```

### Using Flags

You can also provide the provider and token as flags:

```bash
./repo-analyzer --provider github --token your-token
```

### Environment Variables

You can set your tokens as environment variables:

```bash
export GITHUB_TOKEN=your-github-token
export GITLAB_TOKEN=your-gitlab-token
```

## Getting a Personal Access Token

### GitHub
1. Go to GitHub Settings > Developer Settings > Personal Access Tokens
2. Generate a new token with the `repo` scope

### GitLab
1. Go to GitLab Settings > Access Tokens
2. Generate a new token with the `read_api` scope

## License

MIT 
