// Package model holds the on-disk names and schema version of the project
// lifecycle state shared by the lifecycle adapters and the composition root.
package model

import ()

// ProjectLifecycleSchemaVersion is the schema version written to the project
// lifecycle profile.
const ProjectLifecycleSchemaVersion = 1

// ProjectLifecycleProfileFile is the project lifecycle profile file name.
const ProjectLifecycleProfileFile = "project.json"

// DocUpkeepQueueFile is the file name of the documentation upkeep queue.
const DocUpkeepQueueFile = "doc-upkeep-queue.jsonl"

// CompactCapsuleFile is the file name of the compaction capsule.
const CompactCapsuleFile = "compact-capsule.json"
