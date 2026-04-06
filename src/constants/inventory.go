package constants

type inventoryData struct {
	Name    string
	Texture string
}

var INVENTORY_ORDER = []string{
	"backpack",
	"enderchest",
	"personal_vault",
	"talisman_bag",
	"potion_bag",
	"fishing_bag",
	"quiver",
	"sacks",
	"museum",
	"rift_inventory",
	"rift_enderchest",
}

var INVENTORY map[string]inventoryData = map[string]inventoryData{
	"backpack":        {Name: "Backpack", Texture: "/api/item/chest"},
	"enderchest":      {Name: "Ender Chest", Texture: "/api/item/ender_chest"},
	"personal_vault":  {Name: "Personal Vault", Texture: "/api/head/f7aadff9ddc546fdcec6ed5919cc39dfa8d0c07ff4bc613a19f2e6d7f2593"},
	"talisman_bag":    {Name: "Talisman Bag", Texture: "/api/head/961a918c0c49ba8d053e522cb91abc74689367b4d8aa06bfc1ba9154730985ff"},
	"potion_bag":      {Name: "Potion Bag", Texture: "/api/head/9f8b82427b260d0a61e6483fc3b2c35a585851e08a9a9df372548b4168cc817c"},
	"fishing_bag":     {Name: "Fishing Bag", Texture: "/api/head/eb8e297df6b8dffcf135dba84ec792d420ad8ecb458d144288572a84603b1631"},
	"quiver":          {Name: "Quiver", Texture: "/api/head/4cb3acdc11ca747bf710e59f4c8e9b3d949fdd364c6869831ca878f0763d1787"},
	"sacks":           {Name: "Sacks", Texture: "/api/head/fb49a2cb90737993201fe72a1f1ab75c5c93c28f047f685ffad5ab20c7b0caf0"},
	"museum":          {Name: "Museum", Texture: "/api/head/438cf3f8e54afc3b3f91d20a49f324dca1486007fe545399055524c17941f4dc"},
	"rift_inventory":  {Name: "Rift Inventory", Texture: "/api/head/445240fcf1a9796327dda5593985343af9121a7156bc76e3d6b341b02e6a6e52"},
	"rift_enderchest": {Name: "Rift Ender Chest", Texture: "/api/head/a6cc486c2be1cb9dfcb2e53dd9a3e9a883bfadb27cb956f1896d602b4067"},
}
