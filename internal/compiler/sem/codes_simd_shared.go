package sem

var cSimdSharedBody = defineCode("E1713", "simd-shared-body", "`{name}` is reached as one shared body for every instantiation, which cannot lower this SIMD operation on `{type}` before an instantiation fixes its lane type and count; call it directly with concrete type arguments, or keep the vector operation out of a body that only a destructor, an interface method table or an erased function value reaches")
