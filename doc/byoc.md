# How to use Bring Your Own Container (BYOC)

Bring Your Own Container is a way to use the Livepeer network with your own runner container. Orchestrators can choose to run the container to provide compute for your workload in a low-touch way that passes the request to the runner container directly and charges based on time of compute.

## Example: Text Reversal Service

This guide demonstrates how to create a simple Python service that reverses text and deploy it as a BYOC capability on the Livepeer network.

### Step 1: Create a Simple Python Text Reversal Server

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

### Step 2: Create a Dockerfile

Create a file named `Dockerfile` in the same directory as your `reverse_server.py`:

````markdown
FROM python:3.9-slim

WORKDIR /app

COPY reverse_server.py .

RUN pip install flask

EXPOSE 5000

CMD ["python", "reverse_server.py"]
````

### Step 3: Build run the Docker Images
```
#build the go-livepeer docker image
#note: price is 0 for the capability registered so will be able to use with wallets with no eth deposit/reserve
docker build --build-arg BUILD_TAGS=mainnet -f docker/Dockerfile -t byoc .
#build the runner
docker build -t reverse-text .


docker run --network host reverse-text
docker run --network host byoc -network arbitrum-one-mainnet -gateway -ethUrl https://arb1.arbitrum.io/rpc -ethPassword secret-password -orchAddr=0.0.0.0:8935 -httpAddr=0.0.0.0:9935 -httpIngest -v=6
docker run --network host byoc -network arbitrum-one-mainnet -orchestrator -ethUrl https://arb1.arbitrum.io/rpc -ethPassword secret-password -pricePerUnit 1 -serviceAddr=0.0.0.0:8935 -orchSecret=orch-secret
```

### Step 4: Register the Capability with local Orchestrator

curl -X POST -k https://localhost:8935/capability/register \
  -H "Authorization: orch-secret" \
  -d '{
    "name": "text-reversal",
    "description": "A service that reverses text input",
    "url": "http://localhost:5000",
    "capacity": 1,
    "price_per_unit": 0,
    "price_scaling": 1,
    "currency": "wei"  
  }'

### Step 5: Send the request to the Gateway

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