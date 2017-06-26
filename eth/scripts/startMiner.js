personal.newAccount("");

primary = eth.accounts[0];
personal.unlockAccount(primary, "", 1500);
miner.setEtherbase(primary);

miner.start(1);
