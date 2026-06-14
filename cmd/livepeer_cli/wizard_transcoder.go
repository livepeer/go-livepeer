func (w *wizard) activateOrchestrator() {
	d, err := w.getDelegatorInfo()
	if err != nil {
		glog.Errorf("Error getting delegator info: %v", err)
		return
	}

	fmt.Printf("Current token balance: %v\n", w.getTokenBalance())
	fmt.Printf("Current bonded amount: %v\n", d.BondedAmount.String())

	val := w.getOrchestratorConfigFormValues()

	if d.BondedAmount.Cmp(big.NewInt(0)) <= 0 || d.DelegateAddress != d.Address {
		fmt.Printf("You must bond to yourself in order to become an orchestrator\n")

		rebond := false

		unbondingLockIDs := w.unbondingLockStats(false)
		if unbondingLockIDs != nil && len(unbondingLockIDs) > 0 {
			fmt.Printf("You have some unbonding locks. Would you like to use one to rebond to yourself? (y/n) - ")

			input := ""
			for {
				input = w.readString()
				if input == "y" || input == "n" {
					break
				}
				fmt.Printf("Enter (y)es or (n)o\n")
			}

			if input == "y" {
				rebond = true

				unbondingLockID := int64(-1)

				for {
					fmt.Printf("Enter the identifier of the unbonding lock you would like to rebond to yourself with - ")
					unbondingLockID = int64(w.readInt())
					if _, ok := unbondingLockIDs[unbondingLockID]; ok {
						break
					}
					fmt.Printf("Must enter a valid unbonding lock ID\n")
				}

				val["unbondingLockId"] = []string{fmt.Sprintf("%v", strconv.FormatInt(unbondingLockID, 10))}
			}
		}

		if !rebond {
			balBigInt, err := lpcommon.ParseBigInt(w.getTokenBalance())
			if err != nil {
				fmt.Printf("Cannot read token balance: %v", w.getTokenBalance())
				return
			}

			amount := big.NewInt(0)
			for amount.Cmp(big.NewInt(0)) == 0 || balBigInt.Cmp(amount) < 0 {
				amount = w.readBigInt("Enter bond amount")
				if balBigInt.Cmp(amount) < 0 {
					fmt.Printf("Must enter an amount smaller than the current balance. ")
				}
				if amount.Cmp(big.NewInt(0)) == 0 && d.BondedAmount.Cmp(big.NewInt(0)) > 0 {
					break
				}
			}

			val["amount"] = []string{fmt.Sprintf("%v", amount.String())}
		}
	}

	result, ok := httpPostWithParams(fmt.Sprintf("http://%v:%v/activateOrchestrator", w.host, w.httpPort), val)
	if !ok {
		fmt.Printf("Error activating orchestrator: %v\n", result)
		return
	}
	// TODO we should confirm if the transaction was actually sent
	fmt.Println("\nTransaction sent. Once confirmed, please restart your node.")
}
