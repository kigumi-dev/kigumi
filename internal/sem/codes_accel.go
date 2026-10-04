package sem

// Two std overloads sharing one runtime dispatch key (package + owner + name, no
// signature): interp's accel tables and the VM/C runtime's std key can't tell them apart.
var cAccelKeyCollision = defineCode("E1131", "accel-key-collision", "`{name}` shares its runtime dispatch key with `{other}`, which has a different signature; give one of the std overloads a distinct name")
