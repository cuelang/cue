Keep: {
	// This comment is included
	excludedStruct: ExcludedStruct
	excludedInt:    ExcludedStruct
}

// ExcludedStruct is not included in the output.
ExcludedStruct: {
	A: int
}

// ExcludedInt is not included in the output.
ExcludedInt: int
