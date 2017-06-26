cd ../protocol
truffle migrate --network lpTest
cd ..
abigen --abi protocol/LivepeerProtocol.abi --pkg main --type LivepeerProtocol --out livepeerProtocol.go
abigen --abi protocol/LivepeerToken.abi --pkg main --type LivepeerToken --out livepeerToken.go
