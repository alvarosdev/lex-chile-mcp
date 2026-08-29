// Package outputbudget holds the single server-wide output budget shared
// by the BCN and CGR tools (output-budget capability): one constant, no
// per-package duplicates.
package outputbudget

// MaxContentChars is the maximum rendered content body (~25K tokens) of a
// tool response before the response degrades (map / pagination). ~100K
// chars of legal text.
const MaxContentChars = 100_000
