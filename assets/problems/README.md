# Problem authoring

Every problem lives in its own folder under `assets/problems/`. The app embeds
this folder, copies any missing folders into the user's data directory on
startup, and discovers problems automatically. No Go code needs to change when
you add a question.

## Folder layout

```
assets/problems/
  017_contains_duplicate/
    problem.json      required
    description.md    optional, preferred for long statements
```

Use a folder name of `<id>_<slug>`:

- `id` is the three-digit display ID (`017`).
- `slug` is the snake-case problem name (`contains_duplicate`).
- The loader resolves a problem by ID (`017`) or by folder name
  (`017_contains_duplicate`), so the name is enough for normal use.
- Problems are sorted by the `id` field, not by folder name.

## Runtime copy

On Windows the editable problem folder is:

```
%LOCALAPPDATA%\openleet\problems
```

On Linux/macOS it is:

```
~/.local/share/openleet/problems
```

The embedded samples are copied there only when a folder does not already
exist. This means:

- A brand-new `assets/problems/<folder>` appears on the next launch.
- Editing an existing embedded sample will **not** overwrite the user's copy.
  Delete that folder from the data directory (or edit the data copy) to pick up
  changes.
- Adding a folder directly to the data directory works without rebuilding the
  app.

## problem.json

Minimal shape:

```json
{
  "id": "017",
  "slug": "contains_duplicate",
  "title": "Contains Duplicate",
  "category": "array",
  "difficulty": "Easy",
  "topics": ["array", "hash-table"],
  "companies": ["amazon"],
  "acceptance_rate": 61.2,
  "templates": {
    "python": "class Solution:\n    def containsDuplicate(self, nums: list[int]) -> bool:\n        pass\n",
    "go": "package main\n\nfunc containsDuplicate(nums []int) bool {\n    return false\n}\n"
  },
  "test_cases": [
    {
      "id": "tc1",
      "input": {"nums": [1, 2, 3, 1]},
      "expected": true,
      "is_hidden": false
    },
    {
      "id": "tc2",
      "input": {"nums": [1, 2, 3, 4]},
      "expected": false,
      "is_hidden": false
    }
  ]
}
```

### Fields

| Field | Required | Notes |
| :--- | :--- | :--- |
| `id` | yes | Display ID; also controls sort order and the roadmap card number. |
| `slug` | yes | Stable machine name, usually snake_case. |
| `title` | yes | Display title shown in the roadmap, editor, and breadcrumb. |
| `category` | no | Roadmap section. Missing means `dsa`. |
| `difficulty` | yes | `Easy`, `Medium`, or `Hard`. |
| `topics` | yes | Used by the inspector and `/topic` filters. |
| `companies` | no | Shown in the inspector; used by `/company`. |
| `acceptance_rate` | no | Display percentage only. |
| `templates` | yes | Starter buffer per language: `python`, `go`, and optional others. |
| `test_cases` | yes | At least one fixture. |
| `scale_test_cases` | no | Optional fixture groups (small, medium, large) for algorithmic curve benchmarking. |
| `description` | no | Markdown statement. `description.md` is preferred. |

`description.md` takes precedence over a `description` field in JSON. Keep the
statement in Markdown so headings, code blocks, examples, and constraints render
through Glamour.

### Categories

The category selects the roadmap rail:

| `category` | Rail |
| :--- | :--- |
| `dsa` or omitted | DSA |
| `cp` | CP |
| `dp` | DP |
| `array` | ARRAY |
| `link_list` | LINK LIST |
| `graph` | GRAPH |

Spacing, hyphens, and case do not matter (`"link list"`, `"link-list"`, and
`"LINK_LIST"` all match).

### Test case inputs

`input` is passed to the solution as keyword arguments when it is an object:

```json
"input": {"nums": [2, 7, 11, 15], "target": 9}
```

When it is an array the values are passed positionally:

```json
"input": [2, 7]
```

`expected` is compared to the returned value. Set `is_hidden: true` for a
fixture that challenges the user without being shown in the visible examples.

### Scale test cases

Problems can optionally supply `scale_test_cases` grouped by input size. The
scaling runner requires at least a `small` and a `large` group; `medium` is
optional but recommended. Each group is an array of the same fixture shape used
by `test_cases`:

```json
"scale_test_cases": {
  "small": [
    {"id": "small_1", "input": {"n": 10}, "expected": 55}
  ],
  "medium": [
    {"id": "medium_1", "input": {"n": 1000}, "expected": 500500}
  ],
  "large": [
    {"id": "large_1", "input": {"n": 100000}, "expected": 5000050000}
  ]
}
```

The example above is for a `sum_to_n` problem, not a fixed size rule. Use
whatever argument shape the problem actually needs; the runner measures only the
function call, not interpreter startup or fixture generation. Keep the
`scale_test_cases` inputs larger than the ordinary `test_cases` so the growth
ratio is meaningful.

### Starter templates

Template keys must match the language IDs the editor uses:

- `python`
- `go`

Other keys are accepted by the loader and appear in the language picker when the
problem is opened.

## Adding a problem

1. Copy `assets/problems/017_contains_duplicate` to a new
   `<id>_<slug>` folder.
2. Change the `id`, `slug`, `title`, `category`, difficulty, topics, templates,
   and test fixtures.
3. Rewrite `description.md`. Include examples and constraints.
4. Build or run the app. The new folder is extracted automatically.
5. If the app is already running against an older data directory, restart it so
   the new folder is copied.

For an editor schema, see `assets/problems/problem.schema.json`.
