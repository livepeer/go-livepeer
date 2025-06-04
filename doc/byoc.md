# How to use Bring Your Own Container (BYOC)

Bring Your Own Container is a way to use the Livepeer network with your own runner container. Orchestrators can choose to run the container to provide compute for your workload in a low-touch way that passes the request to the runner container directly and charges based on time of compute.

## Example: Text Reversal Service

This guide demonstrates how to create a simple Python service that reverses text and deploy it as a BYOC capability on the Livepeer network.

### Step 1a: Create a Simple Python Text Reversal Server

Create a file named `reverse_server.py`:

```python
from flask import Flask, request, Response
import json

app = Flask(__name__)

@app.route('/reverse-text', methods=['POST'])
def reverse_text():
    # Get the text from the request
    content = request.get_json(silent=True) or {}
    text = content.get('text', '')
    
    # Reverse the text
    reversed_text = text[::-1]
    
    # Return the reversed text
    return Response(
        json.dumps({'original': text, 'reversed': reversed_text}),
        mimetype='application/json'
    )

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5000)
```
### Step 1b: Create a container to run external capability

Create a file named `Dockerfile.reverse_server` in the same directory as your `reverse_server.py`:

````markdown
FROM python:3.11-slim

RUN pip install flask

WORKDIR /app

COPY reverse_server.py .

EXPOSE 5000

CMD ["python", "reverse_server.py"]

````

### Step 2a: Create a script to register the external capability with the Orchestrator

Create a file named `register_capability.py`:

```python
import requests, time

#note: price is 0 for the capability registered so will be able to use with wallets with no eth deposit/reserve
#      if price is set above 0 will need to use an on chain Orchestrator and a Gateway with a Deposit/Reserve
data = {
   "name": "text-reversal",
   "url": "http://byoc_reverse_text:5000",
   "capacity": 1,
   "price_per_unit": 0,
   "price_scaling": 1,
   "currency": "wei"
}

headers = {
    "Authorization": "orch-secret"
}

for i in range(10):
    #wait 1 second then try
    time.sleep(1)

    try:
        registered = requests.post("https://byoc_orchestrator:8935/capability/register", json=data, headers=headers, verify=False)
        if registered.status_code == 200:
            break
        else:
            print(f"registration not completed: {registered.text}")
    except Exception as e:
        pass   
```

### Step 2b: Create a container to register external capability

Create a file named `Dockerfile.register_capability` in the same directory as your `register_capability.py`:

````markdown
FROM python:3.11-slim

RUN pip install requests

WORKDIR /app

COPY register_capability.py .

CMD ["python", "register_capability.py"]

````


### Step 3: Build the Docker Images
```
#build the go-livepeer docker image
docker build --build-arg BUILD_TAGS=mainnet -f docker/Dockerfile -t byoc_livepeer .
#build the runner
docker build -f Dockerfile.reverse_server -t byoc_reverse_text .
#build the runner to register the capability
docker build -f Dockerfile.register_capability -t byoc_register_capability .

```

### Step 4: Create docker compose file for the go-livepeer gateway/orchestrator, runner and to register the capability

Create a file named `docker-compose.yml`
```
services:
  orchestrator:
    image: byoc_livepeer
    container_name: byoc_orchestrator
    command: "-network arbitrum-one-mainnet -orchestrator -ethUrl https://arb1.arbitrum.io/rpc -ethPassword secret-password -pricePerUnit 1 -serviceAddr=byoc_orchestrator:8935 -orchSecret=orch-secret -v 6"
  gateway:
    image: byoc_livepeer
    container_name: byoc_gateway
    command: "-network arbitrum-one-mainnet -gateway -ethUrl https://arb1.arbitrum.io/rpc -ethPassword secret-password -orchAddr=byoc_orchestrator:8935 -httpAddr=0.0.0.0:9935 -httpIngest -v 6"
    depends_on:
      - orchestrator
  reverse_text:
    image: byoc_reverse_text
    container_name: byoc_reverse_text
    ports:
      - 5000:5000
    depends_on:
      - orchestrator
  register_capability:
    image: byoc_register_capability
    container_name: byoc_register_capability
    depends_on: 
      - reverse_text

networks:
  default:
    name: byoc_livepeer_network

```

Start the containers in the compose file with:
`docker compose up`

### Step 5: Start
### Step 5: Send the request to the Gateway
Send request to Gateway when the Gateway, Orchestrator and capability runner is up and registered with the Orchestrator.

```
curl -X POST http://localhost:9935/process/request/reverse-text \
  -H "Content-Type: application/json" \
  -H "Livepeer: eyJyZXF1ZXN0IjogIntcInJ1blwiOiBcImVjaG9cIn0iLCAiY2FwYWJpbGl0eSI6ICJ0ZXh0LXJldmVyc2FsIiwgInRpbWVvdXRfc2Vjb25kcyI6IDMwfQ==" \
  -d '{"text":"Hello, Livepeer BYOC!"}'
```
understanding the headers of the request
`Livepeer` header is the base64 encoded json of
```
{
  "request": "{\"run\":\"echo\"}", // JSON string of the request
  "capability": "text-reversal", // The capability name registered with the orchestrator
  "timeout_seconds": 30, // How long the request should wait for a response
}
```

Response will be:
```
{
  "original": "Hello, Livepeer BYOC!",
  "reversed": "!COYB reepevil ,olleH"
}
```