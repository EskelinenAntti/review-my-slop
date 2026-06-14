---
name: comments
description: Retrieve and address code review feedback created with review-my-slop. Use when the user invokes /comments or asks to check, fix, or respond to pending review-my-slop comments in the current Git repository.
---

# Comments

- Run `review-my-slop comments`, address every returned comment, and verify the
  changes. Successful output consumes those comments.
- Run the command again and repeat until it reports no pending comments, since
  new feedback may arrive while you work.
