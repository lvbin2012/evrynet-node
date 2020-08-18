# PUPPETH   

Because puppeth will read SC file at `consensus/staking_contracts/EvrynetStaking.sol` to compile Bytecode & ABI, so we must install `solc` to support. Here is the way to install `solc` on MacOS:
- Firstly, you must check what is SC version you want to compile (we use version 0.5.11 as default)
- Then you'll need to find the specific commit corresponding to your version of this file [Here](https://github.com/ethereum/homebrew-ethereum/commits/master/solidity.rb)
- Use `brew` command with your selected file
```brew install <your_solidity.rb>```   
Ex: `brew install https://raw.githubusercontent.com/ethereum/homebrew-ethereum/7fa7027f20cca27f76c679d0c5b35ee3c565f284/solidity.rb`
- After installing successfully, you can check by get solc version `solc --version`