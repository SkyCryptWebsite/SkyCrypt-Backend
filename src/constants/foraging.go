package constants

var MAX_CENTER_OF_THE_FOREST_LEVEL = 5

var MAX_FISH_FAMILY_AMOUNT = 31

var MAX_HINA_CHAPTER = 7

var MAX_TREE_GIFT_MILESTONE = 7

type Reward struct {
	TokenOfTheMountain int
}

var HOTF_REWARDS = map[int]Reward{
	1: {TokenOfTheMountain: 1},
	2: {TokenOfTheMountain: 2},
	3: {TokenOfTheMountain: 2},
	4: {TokenOfTheMountain: 2},
	5: {TokenOfTheMountain: 2},
	6: {TokenOfTheMountain: 2},
	7: {TokenOfTheMountain: 2},
}

var COTF_REWARDS = map[int]Reward{
	1: {TokenOfTheMountain: 1},
	2: {},
	3: {},
	4: {},
	5: {TokenOfTheMountain: 1},
}
