# review-my-slop

review-my-slop presents Git changes for interactive review and collects review comments.

## Language

**Patch**:
The Git change set being reviewed.
_Avoid_: Diff, change set

**View**:
The rendered presentation of a Patch, in unified or side-by-side form.
_Avoid_: Rendered view, diff screen

**View state**:
The cursor, selection, and viewport associated with a View; an empty View state has no active cursor or selection.
_Avoid_: Reading state

**Preserving**:
Replacing a View while retaining cursor context, valid selection, and relative viewport placement when possible.
_Avoid_: Rebinding
