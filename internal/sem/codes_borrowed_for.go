package sem

var cForMovesBorrowed = defineCode("E2200", "for-moves-borrowed-elements", "`for` over `{name}` would take each element out of a place that is only borrowed, and `{type}` is not known to be `Copy`; iterate `&{name}` to borrow the elements, or take the owner by `move`")
