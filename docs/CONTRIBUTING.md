# Contributing to SampMail

Thank you for your interest in contributing to SampMail! This document provides guidelines and information for contributors.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Making Changes](#making-changes)
- [Pull Request Process](#pull-request-process)
- [Coding Standards](#coding-standards)
- [Testing](#testing)
- [Documentation](#documentation)

---

## Code of Conduct

We are committed to providing a welcoming and inclusive experience for everyone. Please read and follow our Code of Conduct:

- Be respectful and inclusive
- Provide constructive feedback
- Focus on what is best for the community
- Show empathy towards other community members

---

## Getting Started

### Ways to Contribute

- **Bug Reports**: Found a bug? Open an issue with detailed reproduction steps
- **Feature Requests**: Have an idea? Open an issue to discuss it
- **Documentation**: Help improve our docs
- **Code**: Fix bugs or implement features
- **Testing**: Help test pre-release versions
- **Translations**: Help translate the UI

### Finding Issues to Work On

Look for issues labeled:
- `good first issue` - Great for newcomers
- `help wanted` - We need help with these
- `bug` - Confirmed bugs
- `enhancement` - Feature requests

---

## Development Setup

### Prerequisites

- Go 1.21+
- Bun 1.0+
- Git
- Docker (for Reacher)

### Clone and Setup

```bash
# Fork the repository on GitHub, then:
git clone https://github.com/YOUR_USERNAME/sampmail.git
cd sampmail

# Add upstream remote
git remote add upstream https://github.com/pulak-ranjan/sampmail.git

# Install Go dependencies
go mod download

# Install frontend dependencies (using Bun)
cd web
bun install
cd ..
```

### Environment Setup

```bash
# Create development environment file
cat > .env.dev << 'EOF'
SAMPMAIL_SECRET=dev-secret-key-for-local-testing-only
SAMPMAIL_LISTEN_ADDR=127.0.0.1:9000
SAMPMAIL_DATA_DIR=./data
SAMPMAIL_ENV=development
EOF

# Create data directory
mkdir -p data
```

### Running Locally

**Backend (with hot reload):**
```bash
# Install air for hot reload
go install github.com/cosmtrek/air@latest

# Run
air
```

**Frontend (development server):**
```bash
cd web
bun run dev
```

Access the app at `http://localhost:5173` (frontend dev server proxies to backend).

---

## Making Changes

### Branch Naming

Use descriptive branch names:
- `feature/template-editor` - New features
- `fix/campaign-crash` - Bug fixes
- `docs/api-reference` - Documentation
- `refactor/campaign-service` - Code refactoring

### Commit Messages

Follow conventional commits:

```
type(scope): description

[optional body]

[optional footer]
```

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation
- `style`: Formatting (no code change)
- `refactor`: Code restructuring
- `test`: Adding tests
- `chore`: Maintenance

**Examples:**
```
feat(campaigns): add A/B testing support

Implemented variant system for campaigns with automatic
winner selection based on open rates.

Closes #123
```

```
fix(auth): prevent timing attack on login

Use constant-time comparison for password verification.
```

### Keep Changes Focused

- One feature or fix per PR
- Keep PRs reasonably sized (<500 lines ideally)
- Split large changes into multiple PRs

---

## Pull Request Process

### Before Submitting

1. **Sync with upstream:**
   ```bash
   git fetch upstream
   git rebase upstream/main
   ```

2. **Run tests:**
   ```bash
   go test ./...
   cd web && npm run lint
   ```

3. **Check formatting:**
   ```bash
   go fmt ./...
   goimports -w .
   ```

4. **Update documentation** if needed

### Submitting

1. Push to your fork:
   ```bash
   git push origin feature/your-feature
   ```

2. Open a Pull Request on GitHub

3. Fill out the PR template:
   - Description of changes
   - Related issues
   - Testing performed
   - Screenshots (for UI changes)

### Review Process

1. Automated checks must pass (CI, linting, tests)
2. At least one maintainer review required
3. Address feedback in new commits (don't force-push during review)
4. Squash commits when merging

---

## Coding Standards

### Go

Follow the [Effective Go](https://golang.org/doc/effective_go) guidelines.

**Formatting:**
```bash
gofmt -s -w .
goimports -w .
```

**Linting:**
```bash
golangci-lint run
```

**Key Guidelines:**
- Use meaningful variable names
- Keep functions focused and short
- Handle errors explicitly
- Add comments for exported functions
- Use structured logging

**Example:**
```go
// CreateCampaign creates a new email campaign.
// It validates the input, generates necessary IDs, and stores
// the campaign in the database.
func (s *CampaignService) CreateCampaign(ctx context.Context, req CreateCampaignRequest) (*Campaign, error) {
    log := logger.WithComponent("campaign")
    
    // Validate input
    if err := req.Validate(); err != nil {
        return nil, fmt.Errorf("validation failed: %w", err)
    }
    
    campaign := &Campaign{
        Name:     req.Name,
        Subject:  req.Subject,
        Status:   StatusDraft,
        CreatedAt: time.Now(),
    }
    
    if err := s.store.Create(campaign); err != nil {
        log.Error("failed to create campaign", "error", err)
        return nil, fmt.Errorf("database error: %w", err)
    }
    
    log.Info("campaign created", "id", campaign.ID, "name", campaign.Name)
    return campaign, nil
}
```

### React/TypeScript

**Formatting:**
```bash
bun run lint
bun run type-check
```

**Key Guidelines:**
- Use functional components with hooks
- Use TypeScript for ALL components
- Keep components small and focused
- Use Tailwind for styling
- Extract reusable logic into hooks
- Use path aliases (`@/components`, `@/lib`)

**Example:**
```tsx
interface CampaignCardProps {
  campaign: Campaign;
  onEdit: (id: number) => void;
  onDelete: (id: number) => void;
}

export function CampaignCard({ campaign, onEdit, onDelete }: CampaignCardProps) {
  const [isDeleting, setIsDeleting] = useState(false);
  
  const handleDelete = async () => {
    setIsDeleting(true);
    try {
      await onDelete(campaign.id);
    } finally {
      setIsDeleting(false);
    }
  };
  
  return (
    <div className="bg-white rounded-lg shadow p-4">
      <h3 className="font-semibold text-lg">{campaign.name}</h3>
      <p className="text-gray-500 text-sm">{campaign.subject}</p>
      <div className="flex gap-2 mt-4">
        <Button onClick={() => onEdit(campaign.id)}>Edit</Button>
        <Button 
          variant="danger" 
          onClick={handleDelete}
          disabled={isDeleting}
        >
          {isDeleting ? 'Deleting...' : 'Delete'}
        </Button>
      </div>
    </div>
  );
}
```

---

## Testing

### Backend Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific package
go test ./internal/core/...

# Verbose output
go test -v ./...
```

**Writing Tests:**
```go
func TestCampaignService_Create(t *testing.T) {
    // Setup
    store := newTestStore(t)
    service := NewCampaignService(store)
    
    // Test
    campaign, err := service.Create(context.Background(), CreateRequest{
        Name: "Test Campaign",
        Subject: "Test Subject",
    })
    
    // Assert
    require.NoError(t, err)
    assert.Equal(t, "Test Campaign", campaign.Name)
    assert.Equal(t, StatusDraft, campaign.Status)
}
```

### Frontend Tests

```bash
cd web

# Type check
bun run type-check

# Lint
bun run lint

# Build (catches errors)
bun run build
```

---

## Documentation

### Code Documentation

- Add doc comments to all exported functions
- Include examples for complex APIs
- Keep docs up-to-date with code changes

### User Documentation

- Update relevant docs when adding features
- Include screenshots for UI changes
- Write clear, concise instructions

### API Documentation

When adding/changing endpoints:
1. Update `docs/API.md`
2. Include request/response examples
3. Document error cases

---

## Questions?

- Open a [GitHub Discussion](https://github.com/pulak-ranjan/sampmail/discussions)
- Check existing issues and PRs
- Read the documentation

Thank you for contributing! 🎉
