package test

//These contants required test environment. The params bellow are the address on testNode
const (
	// this is just a random address to run negative test
	normalAddress                  = "0x7e8A370FD68DeBe1f6B5f3537061F1069BAC783C"
	senderPK                       = "ce900e4057ef7253ce737dccf3979ec4e74a19d595e8cc30c6c5ea92dfdd37f1"
	senderAddrStr                  = "0x560089ab68dc224b250f9588b3db540d87a66b7a"
	contractAddrStrWithoutProvider = "0x7Dadf7b6c9C828afd6E814cDb3e820f9F3261e49"
	contractAddrStrWithProvider    = "0xe09faac3574aa5f498b8006b0974aace2ee90b06"

	providerPK         = "ce900e4057ef7253ce737dccf3979ec4e74a19d595e8cc30c6c5ea92dfdd37f1"
	providerAddrStr    = "0x560089ab68dc224b250f9588b3db540d87a66b7a"
	invadlidProviderPK = "5564a4ddd059ba6352aae637812ea6be7d818f92b5aff3564429478fcdfe4e8a"

	// payload to create a smart contract
	payload = "0x608060405260d0806100126000396000f30060806040526004361060525763ffffffff7c01000000000000000000000000000000000000000000000000000000006000350416633fb5c1cb811460545780638381f58a14605d578063f2c9ecd8146081575b005b60526004356093565b348015606857600080fd5b50606f6098565b60408051918252519081900360200190f35b348015608c57600080fd5b50606f609e565b600055565b60005481565b600054905600a165627a7a723058209573e4f95d10c1e123e905d720655593ca5220830db660f0641f3175c1cdb86e0029"

	providerWithoutGasAddr     = "0x6D49e3Ba3f77a19e9dF7EceD8AA7154Fe372ea27"
	providerWithoutGasPK       = "34b377a903b4a01c55062d978160084992271c4f89797caa18fd4e1d61123fbb"
	contractProviderWithoutGas = "0xd8bC6551dCc074f845ba036b84174ec9276C4a37"

	senderWithoutGasPK      = "AEC5EB6A80CC094363D206949C3ED475C2C5060A23049150310D4FD39F95AF99"
	senderWithoutGasAddrStr = "0xb61F4c3E676cE9f4FbF7f5597A303eEeC3AE531B"

	testGasLimit   = 1000000
	testGasPrice   = 1000000000
	testAmountSend = 1000000000
	ethRPCEndpoint = "http://localhost:8545"
)
