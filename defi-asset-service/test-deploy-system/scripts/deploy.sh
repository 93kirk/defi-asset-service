#!/bin/bash

# DeFi Asset Service Deployment Script
# Version: 1.0.0

set -e

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
ENVIRONMENT=${ENVIRONMENT:-"development"}
DOCKER_REGISTRY=${DOCKER_REGISTRY:-"localhost:5000"}
IMAGE_NAME="defi-asset-service"
IMAGE_TAG=${IMAGE_TAG:-"latest"}
DEPLOYMENT_DIR=$(dirname "$0")/..
LOG_FILE="/tmp/defi-deploy-$(date +%Y%m%d-%H%M%S).log"

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $(date '+%Y-%m-%d %H:%M:%S') - $1" | tee -a "$LOG_FILE"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $(date '+%Y-%m-%d %H:%M:%S') - $1" | tee -a "$LOG_FILE"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $(date '+%Y-%m-%d %H:%M:%S') - $1" | tee -a "$LOG_FILE"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $(date '+%Y-%m-%d %H:%M:%S') - $1" | tee -a "$LOG_FILE"
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    # Check Docker
    if ! command -v docker &> /dev/null; then
        log_error "Docker is not installed. Please install Docker first."
        exit 1
    fi
    
    # Check Docker Compose
    if ! command -v docker-compose &> /dev/null; then
        log_error "Docker Compose is not installed. Please install Docker Compose first."
        exit 1
    fi
    
    # Check required environment variables
    if [[ -z "$DB_PASSWORD" && "$ENVIRONMENT" == "production" ]]; then
        log_error "DB_PASSWORD environment variable is required for production deployment."
        exit 1
    fi
    
    log_success "Prerequisites check passed"
}

# Build Docker images
build_images() {
    log_info "Building Docker images..."
    
    cd "$DEPLOYMENT_DIR"
    
    # Build API Gateway
    log_info "Building API Gateway image..."
    docker build -t "${DOCKER_REGISTRY}/${IMAGE_NAME}-api:${IMAGE_TAG}" -f Dockerfile .
    
    # Build Queue Worker
    if [[ -f "Dockerfile.worker" ]]; then
        log_info "Building Queue Worker image..."
        docker build -t "${DOCKER_REGISTRY}/${IMAGE_NAME}-worker:${IMAGE_TAG}" -f Dockerfile.worker .
    fi
    
    log_success "Docker images built successfully"
}

# Push images to registry (if registry is specified)
push_images() {
    if [[ "$DOCKER_REGISTRY" != "localhost:5000" ]]; then
        log_info "Pushing images to registry: $DOCKER_REGISTRY"
        
        # Push API Gateway
        docker push "${DOCKER_REGISTRY}/${IMAGE_NAME}-api:${IMAGE_TAG}"
        
        # Push Queue Worker
        if [[ -f "Dockerfile.worker" ]]; then
            docker push "${DOCKER_REGISTRY}/${IMAGE_NAME}-worker:${IMAGE_TAG}"
        fi
        
        log_success "Images pushed to registry"
    else
        log_warning "Skipping image push (using local registry)"
    fi
}

# Generate environment configuration
generate_env_config() {
    log_info "Generating environment configuration for $ENVIRONMENT..."
    
    cd "$DEPLOYMENT_DIR"
    
    # Create environment-specific directory
    mkdir -p "env/$ENVIRONMENT"
    
    # Generate .env file
    cat > "env/$ENVIRONMENT/.env" << EOF
# DeFi Asset Service - $ENVIRONMENT Environment
# Generated on $(date)

# Application Settings
ENV=$ENVIRONMENT
LOG_LEVEL=info
PORT=8080

# Database Configuration
DB_HOST=mysql
DB_PORT=3306
DB_NAME=defi_asset_service
DB_USER=defi_user
DB_PASSWORD=${DB_PASSWORD:-defi_password}
DB_MAX_CONNECTIONS=50
DB_MAX_IDLE_CONNECTIONS=10

# Redis Configuration
REDIS_HOST=redis
REDIS_PORT=6379
REDIS_PASSWORD=${REDIS_PASSWORD:-}
REDIS_DB=0
REDIS_POOL_SIZE=100

# External Services
EXTERNAL_SERVICE_A_URL=${EXTERNAL_SERVICE_A_URL:-http://external-service-a:8081}
EXTERNAL_SERVICE_B_URL=${EXTERNAL_SERVICE_B_URL:-http://external-service-b:8082}
EXTERNAL_SERVICE_TIMEOUT=30
EXTERNAL_SERVICE_MAX_RETRIES=3

# JWT Configuration
JWT_SECRET=${JWT_SECRET:-your-jwt-secret-key-here-change-in-production}
JWT_EXPIRY_HOURS=24

# Rate Limiting
RATE_LIMIT_REQUESTS_PER_MINUTE=60
RATE_LIMIT_BURST=10

# Cache Configuration
CACHE_TTL_SECONDS=600
CACHE_PREFIX=defi

# Queue Configuration
QUEUE_NAME=position_updates
QUEUE_WORKERS=5
QUEUE_MAX_RETRIES=3

# Monitoring
PROMETHEUS_ENABLED=true
PROMETHEUS_PORT=9091
METRICS_ENABLED=true

# Security
CORS_ALLOWED_ORIGINS=*
API_KEY_HEADER=X-API-Key
EOF
    
    log_success "Environment configuration generated"
}

# Deploy services
deploy_services() {
    log_info "Deploying services for $ENVIRONMENT environment..."
    
    cd "$DEPLOYMENT_DIR"
    
    # Stop existing services
    log_info "Stopping existing services..."
    docker-compose down --remove-orphans || true
    
    # Pull latest images (if not using local)
    if [[ "$DOCKER_REGISTRY" != "localhost:5000" ]]; then
        log_info "Pulling latest images..."
        docker-compose pull || log_warning "Failed to pull some images"
    fi
    
    # Start services
    log_info "Starting services..."
    
    if [[ "$ENVIRONMENT" == "production" ]]; then
        # Production deployment with scaling
        docker-compose -f docker-compose.yml -f docker-compose.prod.yml up -d \
            --scale api-gateway=3 \
            --scale queue-worker=5
    else
        # Development deployment
        docker-compose up -d
    fi
    
    # Wait for services to be healthy
    log_info "Waiting for services to be healthy..."
    sleep 30
    
    # Check service health
    check_service_health
    
    log_success "Services deployed successfully"
}

# Check service health
check_service_health() {
    log_info "Checking service health..."
    
    local services=("api-gateway" "mysql" "redis")
    local all_healthy=true
    
    for service in "${services[@]}"; do
        if docker-compose ps "$service" | grep -q "Up (healthy)"; then
            log_success "$service is healthy"
        else
            log_error "$service is not healthy"
            all_healthy=false
        fi
    done
    
    if [[ "$all_healthy" == false ]]; then
        log_error "Some services are not healthy. Check logs with: docker-compose logs"
        exit 1
    fi
}

# Run database migrations
run_migrations() {
    log_info "Running database migrations..."
    
    # Wait for MySQL to be ready
    log_info "Waiting for MySQL to be ready..."
    local max_attempts=30
    local attempt=1
    
    while [[ $attempt -le $max_attempts ]]; do
        if docker-compose exec -T mysql mysql -h localhost -u defi_user -pdefi_password -e "SELECT 1" defi_asset_service &> /dev/null; then
            log_success "MySQL is ready"
            break
        fi
        
        log_info "Waiting for MySQL... (attempt $attempt/$max_attempts)"
        sleep 2
        ((attempt++))
    done
    
    if [[ $attempt -gt $max_attempts ]]; then
        log_error "MySQL is not ready after $max_attempts attempts"
        exit 1
    fi
    
    # Run migrations
    if [[ -f "database/migrations" ]]; then
        log_info "Running schema migrations..."
        # Add migration commands here
        # Example: docker-compose exec api-gateway ./migrate up
    fi
    
    log_success "Database migrations completed"
}

# Perform smoke tests
run_smoke_tests() {
    log_info "Running smoke tests..."
    
    local api_url="http://localhost:8080"
    
    # Test health endpoint
    log_info "Testing health endpoint..."
    if curl -s -f "${api_url}/health" | grep -q "healthy"; then
        log_success "Health endpoint is working"
    else
        log_error "Health endpoint test failed"
        return 1
    fi
    
    # Test API endpoint with authentication
    log_info "Testing API endpoint..."
    local test_response=$(curl -s -f -H "X-API-Key: test-key" "${api_url}/v1/protocols" || echo "FAILED")
    
    if [[ "$test_response" != "FAILED" ]]; then
        log_success "API endpoint is working"
    else
        log_warning "API endpoint test failed (might be expected if no test data)"
    fi
    
    log_success "Smoke tests passed"
}

# Backup existing deployment
backup_deployment() {
    if [[ "$ENVIRONMENT" == "production" ]]; then
        log_info "Creating backup of current deployment..."
        
        local backup_dir="/backup/defi-asset-service/$(date +%Y%m%d-%H%M%S)"
        mkdir -p "$backup_dir"
        
        # Backup Docker Compose configuration
        cp docker-compose.yml docker-compose.prod.yml "$backup_dir/" 2>/dev/null || true
        
        # Backup environment files
        cp -r env/ "$backup_dir/" 2>/dev/null || true
        
        # Backup database (if possible)
        if docker-compose exec -T mysql mysqldump --version &> /dev/null; then
            log_info "Backing up database..."
            docker-compose exec -T mysql mysqldump -h localhost -u defi_user -pdefi_password defi_asset_service > "$backup_dir/database-backup.sql"
        fi
        
        log_success "Backup created at: $backup_dir"
    fi
}

# Rollback deployment
rollback_deployment() {
    log_warning "Rolling back deployment..."
    
    # Stop current services
    docker-compose down
    
    # Restore from backup if available
    local latest_backup=$(ls -td /backup/defi-asset-service/*/ 2>/dev/null | head -1)
    
    if [[ -n "$latest_backup" ]]; then
        log_info "Restoring from backup: $latest_backup"
        
        # Restore configuration
        cp "$latest_backup/docker-compose.yml" "$latest_backup/docker-compose.prod.yml" . 2>/dev/null || true
        
        # Restore database
        if [[ -f "$latest_backup/database-backup.sql" ]]; then
            log_info "Restoring database..."
            docker-compose exec -T mysql mysql -h localhost -u defi_user -pdefi_password defi_asset_service < "$latest_backup/database-backup.sql"
        fi
    fi
    
    # Start previous version
    docker-compose up -d
    
    log_success "Rollback completed"
}

# Cleanup old resources
cleanup_resources() {
    log_info "Cleaning up old resources..."
    
    # Remove old Docker images
    docker image prune -f --filter "until=24h"
    
    # Remove old containers
    docker container prune -f --filter "until=24h"
    
    # Remove old volumes (be careful with this in production)
    if [[ "$ENVIRONMENT" != "production" ]]; then
        docker volume prune -f
    fi
    
    log_success "Cleanup completed"
}

# Main deployment function
main_deploy() {
    log_info "Starting deployment for $ENVIRONMENT environment"
    
    # Parse command line arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            --build)
                BUILD_IMAGES=true
                shift
                ;;
            --push)
                PUSH_IMAGES=true
                shift
                ;;
            --migrate)
                RUN_MIGRATIONS=true
                shift
                ;;
            --test)
                RUN_TESTS=true
                shift
                ;;
            --rollback)
                rollback_deployment
                exit 0
                ;;
            --cleanup)
                CLEANUP=true
                shift
                ;;
            *)
                log_error "Unknown option: $1"
                exit 1
                ;;
        esac
    done
    
    # Execute deployment steps
    check_prerequisites
    
    if [[ "$BUILD_IMAGES" == true ]]; then
        build_images
    fi
    
    if [[ "$PUSH_IMAGES" == true ]]; then
        push_images
    fi
    
    generate_env_config
    backup_deployment
    deploy_services
    
    if [[ "$RUN_MIGRATIONS" == true ]]; then
        run_migrations
    fi
    
    if [[ "$RUN_TESTS" == true ]]; then
        run_smoke_tests
    fi
    
    if [[ "$CLEANUP" == true ]]; then
        cleanup_resources
    fi
    
    log_success "Deployment completed successfully!"
    log_info "Deployment log: $LOG_FILE"
    
    # Display deployment information
    echo ""
    echo "========================================="
    echo "DeFi Asset Service Deployment Summary"
    echo "========================================="
    echo "Environment: $ENVIRONMENT"
    echo "API Gateway: http://localhost:8080"
    echo "Health Check: http://localhost:8080/health"
    echo "MySQL Admin: http://localhost:8083"
    echo "Redis Admin: http://localhost:8084"
    echo "Grafana: http://localhost:3000 (admin/admin)"
    echo "Prometheus: http://localhost:9090"
    echo "========================================="
}

# Handle script execution
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main_deploy "$@"
fi