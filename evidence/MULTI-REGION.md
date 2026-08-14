# Multi-region: the fleet's brain survives the loss of an AWS region

## Database regions
```
{
    "columns": [
        "database",
        "region",
        "primary",
        "secondary",
        "zones"
    ],
    "row_count": 1,
    "rows": [
        [
            "hivemind",
            "aws-ap-southeast-1",
            true,
            false,
            [
                "aws-ap-southeast-1a",
                "aws-ap-southeast-1b",
                "aws-ap-southeast-1c"
            ]
        ]
    ],
    "truncated": false
}
```

## Survival goal
```
{
    "columns": [
        "database",
        "survival_goal"
    ],
    "row_count": 1,
    "rows": [
        [
            "hivemind",
            "zone"
        ]
    ],
    "truncated": false
}
```

## Replica placement
```
{
    "columns": [
        "ranges",
        "avg_replicas_per_range"
    ],
    "row_count": 1,
    "rows": [
        [
            1,
            3.0
        ]
    ],
    "truncated": false
}
```
