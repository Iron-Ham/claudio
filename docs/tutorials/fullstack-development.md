# Full-Stack Development: Using Claudio with Multi-Service Applications

**Time: 25-35 minutes**

This tutorial explains how to use Claudio effectively with full-stack applications that combine frontend and backend services, using Docker, Docker Compose, and microservices architectures.

## Overview

Claudio's git worktree architecture excels at coordinating full-stack development:

- **Service isolation**: Frontend and backend can be developed independently
- **Container coordination**: Docker containers per worktree avoid port conflicts
- **Database isolation**: Each worktree can use separate database instances
- **API contract enforcement**: Changes can be coordinated across the stack
- **Integration testing**: Each worktree can run full-stack integration tests

## Prerequisites

- Claudio initialized in your project (`claudio init`)
- Docker and Docker Compose installed
- Familiarity with basic Claudio operations (see [Quick Start](quick-start.md))
- Understanding of your stack (React/Vue + Node/Python/Go, etc.)

## Understanding Full-Stack and Git Worktrees

### Typical Full-Stack Structure

```
project/
├── frontend/           # React/Vue/Angular app
│   ├── package.json
│   └── src/
├── backend/            # Node/Python/Go API
│   ├── package.json   (or requirements.txt, go.mod)
│   └── src/
├── docker-compose.yml  # Service orchestration
├── .env                # Environment variables
└── infra/              # Infrastructure configs
```

### Worktree Considerations

Each worktree is a complete copy:

```
.claudio/worktrees/abc123/
├── frontend/           # Independent frontend
├── backend/            # Independent backend
├── docker-compose.yml  # Copy of compose file
└── .env                # Can be customized per worktree
```

**Key insight**: Services in different worktrees need different ports to avoid conflicts.

## Strategy 1: Layer-Based Development (Recommended)

Best for: Most full-stack applications.

### Concept

Assign instances to different layers of the application:

```
Instance 1: Frontend changes
Instance 2: Backend API changes
Instance 3: Database/Infrastructure changes
Instance 4: Integration tests
```

### Workflow

```bash
claudio start fullstack-feature
```

**Task 1 - Backend API:**
```
Implement the user preferences API in backend/src/api/preferences.

1. Create GET /api/preferences endpoint
2. Create PUT /api/preferences endpoint
3. Add request validation
4. Add unit tests

cd backend
npm install
npm run test
npm run start:dev  # Port 4001
```

**Task 2 - Frontend UI:**
```
Implement the preferences page in frontend/src/pages/Preferences.

1. Create PreferencesPage component
2. Add API integration
3. Add form validation
4. Add unit tests

cd frontend
npm install
npm run test
npm run dev -- --port 3001
```

**Task 3 - Integration:**
```
Add E2E tests for the preferences feature.

1. Start both services
2. Write Cypress/Playwright tests
3. Run integration tests

docker-compose up -d
npm run test:e2e
```

## Strategy 2: Docker Compose Per Worktree

Best for: Complex multi-service applications.

### Port Configuration

Create a port offset system for worktrees:

```yaml
# docker-compose.override.yml (per worktree)
services:
  frontend:
    ports:
      - "${FRONTEND_PORT:-3000}:3000"
  backend:
    ports:
      - "${BACKEND_PORT:-4000}:4000"
  postgres:
    ports:
      - "${DB_PORT:-5432}:5432"
```

### Environment Variables

Create `.env` per worktree with unique ports:

```bash
# .claudio/worktrees/abc123/.env
FRONTEND_PORT=3001
BACKEND_PORT=4001
DB_PORT=5433
DB_NAME=app_abc123
```

```bash
# .claudio/worktrees/def456/.env
FRONTEND_PORT=3002
BACKEND_PORT=4002
DB_PORT=5434
DB_NAME=app_def456
```

### Task Instructions

```
Start services with worktree-specific ports:

1. Create .env with:
   FRONTEND_PORT=3001
   BACKEND_PORT=4001
   DB_PORT=5433
   DB_NAME=app_${PWD##*/}

2. docker-compose up -d

3. Implement the feature...
```

## Strategy 3: Microservices Coordination

Best for: Microservices architectures.

### Service Structure

```
project/
├── services/
│   ├── user-service/
│   ├── order-service/
│   ├── payment-service/
│   └── notification-service/
├── gateway/
├── frontend/
└── docker-compose.yml
```

### Task Assignment

Each instance handles a specific service:

**Instance 1 - User Service:**
```
Implement user preferences in services/user-service.

cd services/user-service
docker-compose up -d user-service postgres
npm run test
npm run dev
```

**Instance 2 - Order Service:**
```
Add discount calculation in services/order-service.

cd services/order-service
docker-compose up -d order-service redis
npm run test
npm run dev
```

**Instance 3 - Gateway:**
```
Add new routes to gateway for preferences.

cd gateway
docker-compose up -d gateway
npm run test
npm run dev
```

## Database Strategies

### SQLite (Simplest)

For development, SQLite provides easy isolation:

```
# In .env per worktree
DATABASE_URL=sqlite:./db_${WORKTREE_ID}.sqlite
```

### PostgreSQL with Isolated Databases

Create databases per worktree:

```bash
# Setup script
export DB_NAME="app_$(basename $PWD)"
psql -c "CREATE DATABASE $DB_NAME"
```

Task instruction:
```
Set up isolated database and implement feature:

1. Create database: createdb app_$(basename $PWD)
2. Update .env with DATABASE_URL
3. Run migrations
4. Implement feature
```

### Docker PostgreSQL per Worktree

```yaml
# docker-compose.yml
services:
  postgres:
    image: postgres:15
    environment:
      POSTGRES_DB: ${DB_NAME:-app}
    ports:
      - "${DB_PORT:-5432}:5432"
    volumes:
      - postgres_data_${WORKTREE_ID:-default}:/var/lib/postgresql/data

volumes:
  postgres_data_${WORKTREE_ID:-default}:
```

### MongoDB Isolation

```yaml
services:
  mongo:
    image: mongo:7
    ports:
      - "${MONGO_PORT:-27017}:27017"
    environment:
      MONGO_INITDB_DATABASE: ${DB_NAME:-app}
```

## Testing Strategies

### Unit Tests (Parallel-Safe)

Run independently per service:

**Frontend:**
```
cd frontend
npm test -- --testPathPattern="components"
```

**Backend:**
```
cd backend
npm test -- --testPathPattern="api"
```

### Integration Tests (Need Coordination)

Use isolated environments:

```
Run integration tests with isolated services:

1. Create .env with unique ports
2. docker-compose up -d
3. npm run test:integration
4. docker-compose down
```

### E2E Tests

Coordinate E2E tests across the stack:

**Option A: Sequential Execution**
```bash
claudio add "Run E2E tests" --depends-on "frontend,backend"
```

**Option B: Isolated Environments**
```
Run E2E tests in isolated environment:

1. docker-compose -p e2e_$(basename $PWD) up -d
2. npm run test:e2e
3. docker-compose -p e2e_$(basename $PWD) down
```

### Contract Testing

For API contracts between services:

```
Run contract tests for user-service API:

1. Start mock server with contract
2. Run frontend contract tests
3. Run backend contract tests

npm run test:contract
```

## API Development Workflow

### Backend-First Development

```
# Instance 1: Create API
Implement the new preferences API in backend.

1. Define OpenAPI spec in api/preferences.yaml
2. Generate types from spec
3. Implement endpoints
4. Add tests

# Instance 2: Consume API (depends on Instance 1)
Integrate preferences API in frontend.

1. Generate client from OpenAPI spec
2. Create API service
3. Integrate with components
```

### Frontend-First Development

```
# Instance 1: Create Mock API
Create mock preferences API for frontend development.

1. Create mock server responses
2. Implement frontend features
3. Write frontend tests

# Instance 2: Real API Implementation
Implement actual preferences API matching frontend expectations.

1. Match mock API contract
2. Implement backend logic
3. Run integration tests
```

### Shared Types

For shared TypeScript types:

```
project/
├── shared/
│   └── types/
│       └── preferences.ts
├── frontend/
└── backend/
```

Task instruction:
```
Update shared types before implementing:

1. Edit shared/types/preferences.ts
2. Run type generation for both services
3. Implement feature using new types
```

## Common Conflict Points

### File Conflicts

| File | Risk | Mitigation |
|------|------|------------|
| `docker-compose.yml` | HIGH | One instance for infra changes |
| `.env` | MEDIUM | Usually gitignored |
| `package.json` (both) | MEDIUM | One instance per service |
| Shared types | MEDIUM | Sequential type changes |
| API specs | MEDIUM | One instance owns spec |

### Task Design

**Good decomposition:**
```
├── frontend/ feature changes
├── backend/ API changes
├── shared/ type updates (first)
└── E2E tests (last)
```

**Risky decomposition:**
```
├── Login feature (touches all services)
├── Profile feature (touches all services)
└── Settings feature (touches all services)
```

## Resource Management

### Memory Considerations

Running multiple Docker environments consumes memory:

```yaml
# docker-compose.yml - set memory limits
services:
  backend:
    deploy:
      resources:
        limits:
          memory: 512M
  postgres:
    deploy:
      resources:
        limits:
          memory: 256M
```

### Cleanup Script

Create a cleanup script for worktrees:

```bash
#!/bin/bash
# scripts/cleanup-worktree.sh

# Stop containers
docker-compose down -v

# Remove volumes
docker volume prune -f

# Remove images
docker image prune -f
```

## Example: Complete Feature Development

### Scenario

Implementing a shopping cart feature across:
- Cart API endpoints
- Cart UI components
- Database schema
- E2E tests

### Session Setup

```bash
claudio start cart-feature
```

### Tasks

**Task 1 - Database Schema:**
```
Add cart tables to database schema.

1. Create migration for cart and cart_items tables
2. Add indexes for performance
3. Run migration locally

cd backend
npm run migration:create -- add-cart-tables
npm run migration:run

# Test migration
npm run test:db
```

**Task 2 - Cart API:**
```
Implement cart API endpoints in backend.

Setup:
cd backend
npm install

Implement:
1. POST /api/cart/items - add item to cart
2. GET /api/cart - get cart contents
3. PUT /api/cart/items/:id - update quantity
4. DELETE /api/cart/items/:id - remove item
5. DELETE /api/cart - clear cart

Test:
npm run test -- cart
```

**Task 3 - Cart Service (Frontend):**
```
Create cart service for API integration.

Setup:
cd frontend
npm install

Implement:
1. Create CartService with API client
2. Add CartContext for state management
3. Add useCart hook
4. Add unit tests

Test:
npm run test -- cart
```

**Task 4 - Cart UI:**
```
Implement cart UI components.

Setup:
cd frontend
npm install

Implement:
1. CartIcon with item count badge
2. CartDrawer with item list
3. CartItem component
4. CartSummary with total

Test:
npm run test -- components/cart
```

**Task 5 - E2E Tests:**
```
Add E2E tests for cart functionality.

Setup:
docker-compose up -d
npm run build --prefix frontend
npm run build --prefix backend

Test:
1. Test adding items to cart
2. Test updating quantities
3. Test removing items
4. Test checkout flow

npx playwright test cart.spec.ts
```

## Configuration Recommendations

For full-stack projects:

```yaml
# ~/.config/claudio/config.yaml

# Full-stack builds take time
instance:
  activity_timeout_minutes: 45
  completion_timeout_minutes: 90

# Assign reviewers by area
pr:
  reviewers:
    by_path:
      "frontend/**": [frontend-team]
      "backend/**": [backend-team]
      "shared/**": [tech-lead]
      "docker-compose.yml": [devops]
      "infra/**": [devops]

# Full-stack development can be expensive
resources:
  cost_warning_threshold: 15.00
```

## CI Integration

Example GitHub Actions workflow:

```yaml
name: Full-Stack CI

on:
  pull_request:
    paths:
      - 'frontend/**'
      - 'backend/**'
      - 'shared/**'
      - 'docker-compose.yml'

jobs:
  frontend:
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: ./frontend
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: '20'
          cache: 'npm'
          cache-dependency-path: frontend/package-lock.json
      - run: npm ci
      - run: npm run lint
      - run: npm run test
      - run: npm run build

  backend:
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: ./backend
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: '20'
          cache: 'npm'
          cache-dependency-path: backend/package-lock.json
      - run: npm ci
      - run: npm run lint
      - run: npm run test
      - run: npm run build

  e2e:
    runs-on: ubuntu-latest
    needs: [frontend, backend]
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: '20'
      - name: Start services
        run: docker-compose up -d
      - name: Wait for services
        run: sleep 30
      - name: Run E2E tests
        run: npm run test:e2e
      - name: Stop services
        run: docker-compose down
```

## Troubleshooting

### Port already in use

Another worktree using the same port.

**Solution**:
```bash
# Find what's using the port
lsof -i :3000

# Use different port in .env
FRONTEND_PORT=3005
```

### Database connection refused

Database not running or wrong port.

**Solution**:
```bash
docker-compose ps  # Check if running
docker-compose logs postgres  # Check for errors
```

### Container name conflict

Docker container names must be unique.

**Solution**: Use project name:
```bash
docker-compose -p worktree_$(basename $PWD) up -d
```

### Shared types out of sync

Types don't match between services.

**Solution**:
```bash
# Rebuild shared types
cd shared && npm run build
cd ../frontend && npm install
cd ../backend && npm install
```

### Network conflicts

Docker networks conflicting between worktrees.

**Solution**: Use isolated networks:
```yaml
# docker-compose.yml
networks:
  default:
    name: ${COMPOSE_PROJECT_NAME:-default}_network
```

## What You Learned

- Coordinating frontend and backend development
- Docker Compose isolation strategies
- Database management across worktrees
- Testing strategies for full-stack applications
- API development workflow patterns
- CI integration for multi-service applications

## Next Steps

- [Web Development](web-development.md) - Frontend-specific patterns
- [Monorepo Development](monorepo-development.md) - Large multi-service repos
- [Configuration Guide](../guide/configuration.md) - Customize for your team
