package premine

// this files contains the list of all premine users
// this is just an evaluation for better testing and will probably non exist in the releasing main

// list contains csv value,registration tx line format

import _ "embed"

//go:embed list.txt
var List string
