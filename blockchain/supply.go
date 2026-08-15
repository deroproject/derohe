package blockchain

import "github.com/deroproject/derohe/config"

func CalcSupply(height uint64) uint64 {
	supply := config.PREMINE
	// all registered accounts < block 144000 got an initial balance of 0.002 Dero to claim previous chain balance
	// there were 1358698 such accounts, so total of 271739600 was added to the supply
	supply += 271739600
	remainingBlocks := height
	epochStartHeight := uint64(0)

	for remainingBlocks > 0 {
		blocksInEpoch := RewardReductionInterval
		if remainingBlocks < blocksInEpoch {
			blocksInEpoch = remainingBlocks
		}

		supply += CalcBlockReward(epochStartHeight) * blocksInEpoch

		remainingBlocks -= blocksInEpoch
		epochStartHeight += RewardReductionInterval
	}

	return supply
}
