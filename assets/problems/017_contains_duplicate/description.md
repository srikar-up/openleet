# 17. Contains Duplicate

Given an integer array `nums`, return `true` if any value appears at least twice in the array. Return `false` if every element is distinct.

### Example 1:

```
Input: nums = [1,2,3,1]
Output: true
Explanation: The value 1 appears at indices 0 and 3.
```

### Example 2:

```
Input: nums = [1,2,3,4]
Output: false
Explanation: Every element is distinct.
```

### Example 3:

```
Input: nums = [1,1,1,3,3,4,3,2,4,2]
Output: true
```

### Constraints:

* `1 <= nums.length <= 10^5`
* `-10^9 <= nums[i] <= 10^9`

### Follow-up:

Can you solve it without sorting the array? A hash set lets you detect a repeated value in a single pass.
