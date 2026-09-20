# review-my-slop

review-my-slop supports interactive review of a Patch and collection of its Comments.

## Language

**Patch**:
The Git change set being reviewed, including the repository it originates from.
_Avoid_: Diff, change set

**Patch range**:
A contiguous part of a Patch selected as the subject of review feedback.
_Avoid_: UI selection, screen range

**Review**:
The assessment of a Patch, expressed through its Comments.
_Avoid_: Session

**Comment**:
A piece of review feedback attached to a location in a Patch.
_Avoid_: Message, note

**Anchor**:
The location and quoted context in a Patch to which a Comment is attached.
_Avoid_: Location, selection
