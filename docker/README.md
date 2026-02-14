# Orkes CLI Docker Images

Pre-built Docker images for running stdio workers in any language.

## Available Images

| Image Tag | Runtime | Base Image | Size | Use Case |
|-----------|---------|------------|------|----------|
| `orkes/cli-runner:base` | CLI only | Alpine 3.19 | ~40MB | CLI operations + shell-based workers |
| `orkes/cli-runner:python` | Python 3.12 | Python Alpine | ~100MB | Python stdio workers |
| `orkes/cli-runner:node` | Node.js 20 LTS | Node Alpine | ~180MB | JavaScript/TypeScript workers |
| `orkes/cli-runner:java` | OpenJDK 21 JRE | Temurin Alpine | ~220MB | Java workers |
| `orkes/cli-runner:go` | Go 1.23 | Go Alpine | ~400MB | Go workers |
| `orkes/cli-runner:dotnet` | .NET 8 | .NET Alpine | ~220MB | C# workers |

## Quick Start

### Run a Python worker

```bash
docker run --rm \
  -e CONDUCTOR_SERVER_URL=https://developer.orkescloud.com/api \
  -e CONDUCTOR_AUTH_TOKEN=your-token \
  -v $(pwd)/worker.py:/app/worker.py:ro \
  orkes/cli-runner:python \
  worker stdio --type greeting_task python /app/worker.py
```

### Run with Docker Compose

```yaml
version: '3.8'
services:
  worker:
    image: orkes/cli-runner:python
    command: worker stdio --type my_task python /app/worker.py
    environment:
      CONDUCTOR_SERVER_URL: https://developer.orkescloud.com/api
      CONDUCTOR_AUTH_TOKEN: ${AUTH_TOKEN}
    volumes:
      - ./worker.py:/app/worker.py:ro
```

## Authentication

Three methods supported (in order of precedence):

### 1. Environment Variables (Recommended for containers)

```bash
docker run \
  -e CONDUCTOR_SERVER_URL=https://your-server.com/api \
  -e CONDUCTOR_AUTH_TOKEN=your-token \
  orkes/cli-runner:python worker stdio ...
```

**Available environment variables:**
- `CONDUCTOR_SERVER_URL` - Conductor server URL
- `CONDUCTOR_AUTH_TOKEN` - Authentication token
- `CONDUCTOR_AUTH_KEY` - API key (alternative to token)
- `CONDUCTOR_AUTH_SECRET` - API secret (used with key)

### 2. Mounted Config File

```bash
docker run \
  -v ~/.conductor-cli/config.yaml:/home/orkes/.conductor-cli/config.yaml:ro \
  orkes/cli-runner:python worker stdio ...
```

### 3. Command-line Flags

```bash
docker run orkes/cli-runner:python \
  --server https://your-server.com/api \
  --auth-token your-token \
  worker stdio ...
```

## Worker Examples

### Python Worker (simple_worker.py)

```python
#!/usr/bin/env python3
import sys
import json

task = json.load(sys.stdin)
name = task.get('inputData', {}).get('name', 'World')

result = {
    "status": "COMPLETED",
    "output": {"message": f"Hello, {name}!"},
    "logs": [f"Processed task {task.get('taskId')}"]
}

print(json.dumps(result))
```

### Node.js Worker (simple_worker.js)

```javascript
#!/usr/bin/env node
let input = '';
process.stdin.on('data', chunk => input += chunk);
process.stdin.on('end', () => {
  const task = JSON.parse(input);
  const name = (task.inputData || {}).name || 'World';

  const result = {
    status: 'COMPLETED',
    output: { message: `Hello, ${name}!` },
    logs: [`Processed task ${task.taskId}`]
  };

  console.log(JSON.stringify(result));
});
```

See [`../docker-examples/workers/`](../docker-examples/workers/) for complete examples in all languages.

## Building Images

### Build all images

```bash
./docker/build-images.sh
```

### Build with version tag

```bash
VERSION=v1.0.0 ./docker/build-images.sh
```

### Build and push to registry

```bash
VERSION=v1.0.0 PUSH=true REGISTRY=myregistry ./docker/build-images.sh
```

### Build specific variant

```bash
docker build -f docker/Dockerfile-python -t orkes/cli-runner:python .
```

### Multi-architecture build

```bash
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -f docker/Dockerfile-python \
  -t orkes/cli-runner:python \
  --push .
```

## Testing Images

```bash
./docker/test-images.sh
```

## Advanced Usage

### Installing Additional Dependencies

#### Python packages

```dockerfile
FROM orkes/cli-runner:python
COPY requirements.txt /tmp/
RUN pip install --no-cache-dir -r /tmp/requirements.txt
```

#### Node.js packages

```dockerfile
FROM orkes/cli-runner:node
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production
COPY worker.js .
```

### Running Java/Spring Boot Workers

```dockerfile
FROM eclipse-temurin:21-jdk-alpine AS builder
WORKDIR /build
COPY . .
RUN ./gradlew build --no-daemon

FROM orkes/cli-runner:java
WORKDIR /app
COPY --from=builder /build/build/libs/*.jar app.jar
CMD ["worker", "stdio", "--type", "my_task", "java", "-jar", "app.jar"]
```

Or run directly:

```bash
docker run --rm \
  -e CONDUCTOR_SERVER_URL=https://developer.orkescloud.com/api \
  -e CONDUCTOR_AUTH_TOKEN=your-token \
  -v $(pwd):/workspace \
  -w /workspace \
  orkes/cli-runner:java \
  sh -c './gradlew bootRun'
```

### Using Config Profiles

```bash
docker run \
  -e ORKES_PROFILE=production \
  -v ~/.conductor-cli/config-production.yaml:/home/orkes/.conductor-cli/config-production.yaml:ro \
  orkes/cli-runner:python worker stdio ...
```

### Batch Polling

Process multiple tasks concurrently:

```bash
docker run orkes/cli-runner:python \
  worker stdio --type my_task --count 10 python /app/worker.py
```

### Verbose Logging

Enable detailed logging for debugging:

```bash
docker run orkes/cli-runner:python \
  worker stdio --type my_task --verbose python /app/worker.py
```

## Troubleshooting

### Worker not polling tasks

**Check:**
1. Authentication credentials are correct
2. Server URL is accessible from container
3. Task type matches workflow definition
4. Network connectivity: `docker run orkes/cli-runner:python curl $CONDUCTOR_SERVER_URL`

### Permission denied errors

**Ensure:**
1. Worker files are readable by UID 1000
2. Use read-only mounts (`:ro`) for worker code
3. Don't mount root-owned files without proper permissions

### Container exits immediately

**Check:**
1. Command syntax is correct
2. Worker script has correct shebang (e.g., `#!/usr/bin/env python3`)
3. Worker file has execute permissions
4. View logs: `docker logs <container-id>`

### Runtime errors

**Debug:**
1. Test worker locally first: `echo '{"inputData":{}}' | python worker.py`
2. Use `--verbose` flag to see task and result JSON
3. Check worker logs in task execution output

## Examples

Complete examples available in [`../docker-examples/`](../docker-examples/):

- **workers/** - Worker scripts for all languages
- **docker-compose/** - Docker Compose configurations

## Support

- **Documentation**: https://orkes.io/content
- **GitHub Issues**: https://github.com/conductor-oss/conductor-cli/issues
- **Community**: Conductor Slack

## License

Apache 2.0 - See [LICENSE](../LICENSE) file
