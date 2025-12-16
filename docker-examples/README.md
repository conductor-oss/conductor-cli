# Orkes CLI Docker Examples

Complete examples for deploying Orkes Conductor stdio workers using Docker.

## Directory Structure

```
docker-examples/
├── workers/              # Example worker scripts
│   ├── python/          # Python examples
│   ├── node/            # Node.js examples
│   ├── java/            # Java examples
│   ├── go/              # Go examples
│   └── dotnet/          # .NET examples
├── docker-compose/      # Docker Compose configurations
│   └── docker-compose.yml         # Basic worker example
└── README.md           # This file
```

## Quick Examples

### Docker Run - Python Worker

```bash
cd workers/python
docker run --rm \
  -e CONDUCTOR_SERVER_URL=https://developer.orkescloud.com/api \
  -e CONDUCTOR_AUTH_TOKEN=$AUTH_TOKEN \
  -v $(pwd)/simple_worker.py:/app/worker.py:ro \
  orkes/cli-runner:python \
  worker stdio --type greeting_task python /app/worker.py
```

### Docker Run - Node.js Worker

```bash
cd workers/node
docker run --rm \
  -e CONDUCTOR_SERVER_URL=https://developer.orkescloud.com/api \
  -e CONDUCTOR_AUTH_TOKEN=$AUTH_TOKEN \
  -v $(pwd)/simple_worker.js:/app/worker.js:ro \
  orkes/cli-runner:node \
  worker stdio --type data_transform node /app/worker.js
```

### Docker Run - Go Worker

```bash
cd workers/go
docker run --rm \
  -e CONDUCTOR_SERVER_URL=https://developer.orkescloud.com/api \
  -e CONDUCTOR_AUTH_TOKEN=$AUTH_TOKEN \
  -v $(pwd):/app:ro \
  orkes/cli-runner:go \
  worker stdio --type compute_task go run /app/simple_worker.go
```

## Docker Compose Example

```bash
cd docker-compose
export CONDUCTOR_AUTH_TOKEN=your-token
docker-compose up
```

This runs a simple Python worker.

## Worker Development

### Python Worker with Dependencies

Create a custom image with your dependencies:

```dockerfile
FROM orkes/cli-runner:python
COPY requirements.txt /tmp/
RUN pip install --no-cache-dir -r /tmp/requirements.txt
WORKDIR /app
COPY worker.py .
CMD ["worker", "stdio", "--type", "my_task", "python", "worker.py"]
```

Build and run:

```bash
docker build -t my-python-worker .
docker run --rm \
  -e CONDUCTOR_SERVER_URL=$SERVER_URL \
  -e CONDUCTOR_AUTH_TOKEN=$AUTH_TOKEN \
  my-python-worker
```

### Node.js Worker with Packages

```dockerfile
FROM orkes/cli-runner:node
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production
COPY worker.js .
CMD ["worker", "stdio", "--type", "my_task", "node", "worker.js"]
```

### Java Worker (Compiled)

```dockerfile
FROM orkes/cli-runner:java AS builder
WORKDIR /build
COPY *.java .
RUN javac *.java

FROM orkes/cli-runner:java
WORKDIR /app
COPY --from=builder /build/*.class .
CMD ["worker", "stdio", "--type", "my_task", "java", "SimpleWorker"]
```

### Go Worker (Compiled)

```dockerfile
FROM orkes/cli-runner:go AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 go build -o worker .

FROM orkes/cli-runner:base
COPY --from=builder /build/worker /usr/local/bin/worker
CMD ["worker", "stdio", "--type", "my_task", "/usr/local/bin/worker"]
```

## Common Patterns

### Pattern 1: High-Throughput Processing

Use batch polling to process multiple tasks per poll:

```yaml
services:
  worker:
    image: orkes/cli-runner:python
    command: >
      worker stdio
      --type high_volume_task
      --count 20
      python /app/worker.py
    deploy:
      replicas: 5
```

This configuration:
- Each worker polls 20 tasks at a time
- 5 worker replicas run in parallel
- Total capacity: 100 tasks per polling cycle

### Pattern 2: Multiple Task Types

Run different workers for different task types:

```yaml
services:
  greeting-worker:
    image: orkes/cli-runner:python
    command: worker stdio --type greeting_task python /app/greeting.py

  email-worker:
    image: orkes/cli-runner:python
    command: worker stdio --type send_email python /app/email.py

  transform-worker:
    image: orkes/cli-runner:node
    command: worker stdio --type data_transform node /app/transform.js
```

### Pattern 3: Resource Limits

Set resource limits for workers:

```yaml
services:
  worker:
    image: orkes/cli-runner:go
    command: worker stdio --type compute_intensive go run /app/worker.go
    deploy:
      resources:
        limits:
          cpus: '2.0'
          memory: 2G
        reservations:
          cpus: '1.0'
          memory: 1G
```

### Pattern 4: Health Checks

Add health checks to ensure workers are running:

```yaml
services:
  worker:
    image: orkes/cli-runner:python
    command: worker stdio --type my_task python /app/worker.py
    healthcheck:
      test: ["CMD", "orkes", "--version"]
      interval: 30s
      timeout: 10s
      retries: 3
```

## Development Tips

### 1. Test Workers Locally First

Before containerizing, test your worker locally:

```bash
echo '{"inputData":{"name":"Test"}}' | python worker.py
```

Expected output:
```json
{"status":"COMPLETED","output":{"message":"Hello, Test!"},"logs":["..."]}
```

### 2. Use Verbose Mode for Debugging

Enable verbose output to see task and result JSON:

```bash
docker run ... orkes/cli-runner:python \
  worker stdio --type my_task --verbose python /app/worker.py
```

### 3. Mount Code as Read-Only

Always mount worker code as read-only:

```yaml
volumes:
  - ./worker.py:/app/worker.py:ro  # :ro = read-only
```

### 4. Use Environment-Specific Configs

Use different config files for different environments:

```bash
# Development
docker run -v ~/.conductor-cli/config-dev.yaml:/home/orkes/.conductor-cli/config.yaml:ro ...

# Production
docker run -v ~/.conductor-cli/config-prod.yaml:/home/orkes/.conductor-cli/config.yaml:ro ...
```

### 5. Worker ID for Tracking

Set unique worker IDs for tracking in logs:

```bash
worker stdio --worker-id worker-${HOSTNAME}-${RANDOM} ...
```

## Troubleshooting

### Worker not processing tasks

**Check:**
```bash
# Test connectivity
docker run --rm -e CONDUCTOR_SERVER_URL=$URL orkes/cli-runner:base curl -v $URL

# Verify authentication
docker run --rm \
  -e CONDUCTOR_SERVER_URL=$URL \
  -e CONDUCTOR_AUTH_TOKEN=$TOKEN \
  orkes/cli-runner:base \
  workflow list

# Check task type exists
docker run --rm \
  -e CONDUCTOR_SERVER_URL=$URL \
  -e CONDUCTOR_AUTH_TOKEN=$TOKEN \
  orkes/cli-runner:base \
  task get my_task_type
```

### Worker failing with permission errors

**Fix:**
```bash
# Ensure files are readable by UID 1000
chmod 644 worker.py

# Or change ownership
sudo chown 1000:1000 worker.py
```

### Worker exits immediately

**Debug:**
```bash
# Check logs
docker logs <container-id>

# Run interactively
docker run -it orkes/cli-runner:python sh

# Test command manually
echo '{"inputData":{}}' | docker run -i orkes/cli-runner:python python -
```

### Dependencies missing

**Solution:**
```dockerfile
# Create custom image with dependencies
FROM orkes/cli-runner:python
RUN pip install requests pandas numpy
# ... rest of Dockerfile
```

### Task timeout errors

**Fix:**
```bash
# Increase execution timeout (in seconds)
worker stdio --type my_task --exec-timeout 300 python /app/worker.py
```

## Performance Tuning

### Optimize Batch Size

Experiment with `--count` to find optimal batch size:

```bash
# Small batches (1-5): Low latency, frequent polling
worker stdio --type my_task --count 1 ...

# Medium batches (5-20): Balanced
worker stdio --type my_task --count 10 ...

# Large batches (20-100): High throughput, less frequent polling
worker stdio --type my_task --count 50 ...
```

### Scale Horizontally

Run multiple worker replicas:

```bash
# Docker Compose
docker-compose up --scale worker=10
```

### Resource Allocation

Allocate resources based on workload:

```yaml
# CPU-intensive tasks
resources:
  limits:
    cpus: '4.0'
    memory: 2G

# Memory-intensive tasks
resources:
  limits:
    cpus: '1.0'
    memory: 8G

# I/O-intensive tasks
resources:
  limits:
    cpus: '0.5'
    memory: 512M
```

## Best Practices

1. **Security**
   - Never commit secrets to version control
   - Use environment variables or mounted configs
   - Mount worker code as read-only (`:ro`)
   - Run with non-root user (default in our images)

2. **Reliability**
   - Use `restart: unless-stopped` in Docker Compose
   - Set resource limits to prevent OOM kills
   - Implement proper error handling in workers
   - Use health checks

3. **Observability**
   - Use unique worker IDs for tracking
   - Include detailed logs in worker output
   - Monitor resource usage
   - Set up alerting for failed tasks

4. **Development Workflow**
   - Test workers locally first
   - Use verbose mode for debugging
   - Start with basic examples, then customize
   - Version your worker code

## Next Steps

- Review [worker documentation](../WORKER_STDIO.md) for detailed worker contract
- Check [Docker image documentation](../docker/README.md) for build and deployment
- Join [Conductor Slack](https://orkes.io/slack) for community support

## Support

- **Documentation**: https://orkes.io/content
- **GitHub Issues**: https://github.com/conductor-oss/conductor-cli/issues
- **Community**: Conductor Slack

## License

Apache 2.0 - See [LICENSE](../LICENSE) file
