%rt_type = type { ptr, i32, i32, ptr, i32, ptr, ptr, i64, i64 }
%rt_variant = type { ptr, ptr, i32, i32, [64 x i8] }
%"kg.adt.Result[usize,std/prelude.Error]" = type { i32, [1 x i64] }
%"kg.adt.Result[Void,std/prelude.Error]" = type { i32, [1 x i64] }
declare void @llvm.memset.p0.i64(ptr, i8, i64, i1)
declare void @rt_init(i32, ptr, i32, ptr)
declare void @rt_abi_check(ptr)
declare i32 @rt_exit_code(ptr, ptr, ptr, i64, i64)
declare ptr @rt_unit()
declare ptr @rt_builtin(ptr, i32, ptr)
declare ptr @rt_std_id(i32, i32, ptr)
declare ptr @rt_variant(ptr, i64, ptr)
declare void @rt_cell_init(ptr, ptr)
declare void @rt_drop(ptr)
declare ptr @rt_retain(ptr)
declare void @rt_release(ptr)
@".type.universe.Option" = global %rt_type { ptr @.p0, i32 2, i32 0, ptr null, i32 2, ptr @".type.universe.Option.variants", ptr null, i64 0, i64 0 }
@".variant.universe.Option.Some" = global %rt_variant { ptr @.p1, ptr @".type.universe.Option", i32 0, i32 1, [64 x i8] zeroinitializer }
@".variant.universe.Option.None" = global %rt_variant { ptr @.p2, ptr @".type.universe.Option", i32 1, i32 0, [64 x i8] zeroinitializer }
@".type.universe.Option.variants" = global [2 x ptr] [ptr @".variant.universe.Option.Some", ptr @".variant.universe.Option.None"]
@".type.universe.Result" = global %rt_type { ptr @.p3, i32 2, i32 0, ptr null, i32 2, ptr @".type.universe.Result.variants", ptr null, i64 0, i64 0 }
@".variant.universe.Result.Ok" = global %rt_variant { ptr @.p4, ptr @".type.universe.Result", i32 0, i32 1, [64 x i8] zeroinitializer }
@".variant.universe.Result.Err" = global %rt_variant { ptr @.p5, ptr @".type.universe.Result", i32 1, i32 1, [64 x i8] zeroinitializer }
@".type.universe.Result.variants" = global [2 x ptr] [ptr @".variant.universe.Result.Ok", ptr @".variant.universe.Result.Err"]
@".type.universe.Array" = global %rt_type { ptr @.p6, i32 0, i32 0, ptr null, i32 0, ptr null, ptr null, i64 0, i64 0 }
@".type.std/ffi.CError" = global %rt_type { ptr @.p7, i32 0, i32 2, ptr @".type.std/ffi.CError.fields", i32 0, ptr null, ptr null, i64 0, i64 0 }
@".type.std/ffi.CError.fields" = global [2 x ptr] [ptr @.p8, ptr @.p9]
@".type.std/os.Host" = global %rt_type { ptr @.p10, i32 0, i32 0, ptr null, i32 0, ptr null, ptr null, i64 0, i64 0 }
@".type.std/os.Stdout" = global %rt_type { ptr @.p11, i32 0, i32 0, ptr null, i32 0, ptr null, ptr null, i64 0, i64 0 }
@".type.std/prelude.AllocError" = global %rt_type { ptr @.p12, i32 2, i32 0, ptr null, i32 1, ptr @".type.std/prelude.AllocError.variants", ptr null, i64 0, i64 0 }
@".variant.std/prelude.AllocError.OutOfMemory" = global %rt_variant { ptr @.p13, ptr @".type.std/prelude.AllocError", i32 0, i32 0, [64 x i8] zeroinitializer }
@".type.std/prelude.AllocError.variants" = global [1 x ptr] [ptr @".variant.std/prelude.AllocError.OutOfMemory"]
@rt_types = global [7 x ptr] [ptr @".type.universe.Option", ptr @".type.universe.Result", ptr @".type.universe.Array", ptr @".type.std/ffi.CError", ptr @".type.std/os.Host", ptr @".type.std/os.Stdout", ptr @".type.std/prelude.AllocError"]
define ptr @"kigumi.main"() {
entry:
  %l0 = alloca ptr
  store ptr null, ptr %l0
  %l1 = alloca ptr
  store ptr null, ptr %l1
  %l2 = alloca ptr
  store ptr null, ptr %l2
  %l3 = alloca ptr
  store ptr null, ptr %l3
  %l4 = alloca ptr
  store ptr null, ptr %l4
  %l5 = alloca ptr
  store ptr null, ptr %l5
  %l6 = alloca ptr
  store ptr null, ptr %l6
  %l7 = alloca %"kg.adt.Result[usize,std/prelude.Error]"
  call void @llvm.memset.p0.i64(ptr %l7, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Result[usize,std/prelude.Error]", ptr null, i32 1) to i64), i1 false)
  %l8 = alloca i1
  store i1 0, ptr %l8
  %l9 = alloca ptr
  store ptr null, ptr %l9
  %l10 = alloca %"kg.adt.Result[Void,std/prelude.Error]"
  call void @llvm.memset.p0.i64(ptr %l10, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Result[Void,std/prelude.Error]", ptr null, i32 1) to i64), i1 false)
  %l11 = alloca i64
  store i64 0, ptr %l11
  %l12 = alloca ptr
  store ptr null, ptr %l12
  %l13 = alloca ptr
  store ptr null, ptr %l13
  %l14 = alloca ptr
  store ptr null, ptr %l14
  %l15 = alloca ptr
  store ptr null, ptr %l15
  %l16 = alloca %"kg.adt.Result[usize,std/prelude.Error]"
  call void @llvm.memset.p0.i64(ptr %l16, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Result[usize,std/prelude.Error]", ptr null, i32 1) to i64), i1 false)
  %l17 = alloca i1
  store i1 0, ptr %l17
  %l18 = alloca ptr
  store ptr null, ptr %l18
  %l19 = alloca %"kg.adt.Result[Void,std/prelude.Error]"
  call void @llvm.memset.p0.i64(ptr %l19, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Result[Void,std/prelude.Error]", ptr null, i32 1) to i64), i1 false)
  %l20 = alloca i64
  store i64 0, ptr %l20
  %l21 = alloca ptr
  store ptr null, ptr %l21
  %l22 = alloca ptr
  store ptr null, ptr %l22
  %l23 = alloca ptr
  store ptr null, ptr %l23
  %l24 = alloca ptr
  store ptr null, ptr %l24
  %l25 = alloca ptr
  store ptr null, ptr %l25
  %v1 = alloca ptr, i32 1
  %v2 = alloca ptr, i32 1
  %v3 = alloca [64 x i8], align 16
  %v4 = alloca ptr, i32 1
  %v5 = alloca [64 x i8], align 16
  %v6 = alloca ptr, i32 1
  %v7 = alloca ptr, i32 1
  br label %b0
b0:
  %v8 = call ptr @rt_builtin(ptr @.p14, i32 0, ptr %v1)
  store ptr %v8, ptr %l0
  %v9 = load ptr, ptr %l0
  %v10 = getelementptr ptr, ptr %v2, i32 0
  store ptr %v9, ptr %v10
  %v11 = call ptr @rt_std_id(i32 44, i32 1, ptr %v2)
  store ptr %v11, ptr %l1
  %v12 = load ptr, ptr %l0
  call void @rt_release(ptr %v12)
  %v13 = load ptr, ptr %l1
  store ptr %v13, ptr %l2
  %v14 = load ptr, ptr %l2
  store ptr %v14, ptr %l3
  %v15 = load ptr, ptr %l3
  call void @rt_cell_init(ptr %v3, ptr %v15)
  store ptr %v3, ptr %l4
  store ptr @.p15, ptr %l5
  %v16 = load ptr, ptr %l5
  %v17 = getelementptr ptr, ptr %v4, i32 0
  store ptr %v16, ptr %v17
  %v18 = call ptr @rt_std_id(i32 1, i32 1, ptr %v4)
  store ptr %v18, ptr %l6
  %v19 = load ptr, ptr %l5
  call void @rt_release(ptr %v19)
  %v20 = load ptr, ptr %l4
  %v21 = load ptr, ptr %l6
  %v22 = call %"kg.adt.Result[usize,std/prelude.Error]" @"std/os.Stdout.write"(ptr %v20, ptr %v21)
  store %"kg.adt.Result[usize,std/prelude.Error]" %v22, ptr %l7
  %v23 = load ptr, ptr %l4
  %v24 = getelementptr %"kg.adt.Result[usize,std/prelude.Error]", ptr %l7, i32 0, i32 0
  %v25 = load i32, ptr %v24
  %v26 = icmp eq i32 %v25, 0
  store i1 %v26, ptr %l8
  %v27 = load i1, ptr %l8
  br i1 %v27, label %b1, label %b2
b1:
  %v28 = getelementptr %"kg.adt.Result[usize,std/prelude.Error]", ptr %l7, i32 0, i32 1
  %v29 = load i64, ptr %v28
  store i64 %v29, ptr %l11
  call void @"kg.drop.Result[usize,std/prelude.Error]"(ptr %l7)
  %v30 = load ptr, ptr %l2
  store ptr %v30, ptr %l12
  %v31 = load ptr, ptr %l12
  call void @rt_cell_init(ptr %v5, ptr %v31)
  store ptr %v5, ptr %l13
  store ptr @.p16, ptr %l14
  %v32 = load ptr, ptr %l14
  %v33 = getelementptr ptr, ptr %v6, i32 0
  store ptr %v32, ptr %v33
  %v34 = call ptr @rt_std_id(i32 1, i32 1, ptr %v6)
  store ptr %v34, ptr %l15
  %v35 = load ptr, ptr %l14
  call void @rt_release(ptr %v35)
  %v36 = load ptr, ptr %l13
  %v37 = load ptr, ptr %l15
  %v38 = call %"kg.adt.Result[usize,std/prelude.Error]" @"std/os.Stdout.write"(ptr %v36, ptr %v37)
  store %"kg.adt.Result[usize,std/prelude.Error]" %v38, ptr %l16
  %v39 = load ptr, ptr %l13
  %v40 = getelementptr %"kg.adt.Result[usize,std/prelude.Error]", ptr %l16, i32 0, i32 0
  %v41 = load i32, ptr %v40
  %v42 = icmp eq i32 %v41, 0
  store i1 %v42, ptr %l17
  %v43 = load i1, ptr %l17
  br i1 %v43, label %b3, label %b4
b2:
  %v44 = getelementptr %"kg.adt.Result[usize,std/prelude.Error]", ptr %l7, i32 0, i32 1
  %v45 = load i64, ptr %v44
  %v46 = inttoptr i64 %v45 to ptr
  %v47 = call ptr @rt_retain(ptr %v46)
  store ptr %v47, ptr %l9
  call void @"kg.drop.Result[usize,std/prelude.Error]"(ptr %l7)
  %v48 = load ptr, ptr %l2
  store ptr null, ptr %l2
  call void @rt_drop(ptr %v48)
  call void @llvm.memset.p0.i64(ptr %l10, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Result[Void,std/prelude.Error]", ptr null, i32 1) to i64), i1 false)
  %v49 = getelementptr %"kg.adt.Result[Void,std/prelude.Error]", ptr %l10, i32 0, i32 0
  store i32 1, ptr %v49
  %v50 = getelementptr %"kg.adt.Result[Void,std/prelude.Error]", ptr %l10, i32 0, i32 1
  %v51 = load ptr, ptr %l9
  %v52 = ptrtoint ptr %v51 to i64
  store i64 %v52, ptr %v50
  call void @"kg.retain.Result[Void,std/prelude.Error]"(ptr %l10)
  %v53 = call ptr @"kg.box.Result[Void,std/prelude.Error]"(ptr %l10)
  store ptr %v53, ptr %l24
  call void @"kg.drop.Result[Void,std/prelude.Error]"(ptr %l10)
  %v54 = load ptr, ptr %l24
  ret ptr %v54
b3:
  %v55 = getelementptr %"kg.adt.Result[usize,std/prelude.Error]", ptr %l16, i32 0, i32 1
  %v56 = load i64, ptr %v55
  store i64 %v56, ptr %l20
  call void @"kg.drop.Result[usize,std/prelude.Error]"(ptr %l16)
  store ptr @.p17, ptr %l21
  %v57 = load ptr, ptr %l21
  %v58 = getelementptr ptr, ptr %v7, i32 0
  store ptr %v57, ptr %v58
  %v59 = call ptr @rt_builtin(ptr @.p18, i32 1, ptr %v7)
  store ptr %v59, ptr %l22
  %v60 = load ptr, ptr %l21
  call void @rt_release(ptr %v60)
  %v61 = load ptr, ptr %l22
  call void @rt_drop(ptr %v61)
  %v62 = load ptr, ptr %l2
  store ptr null, ptr %l2
  call void @rt_drop(ptr %v62)
  %v63 = call ptr @rt_unit()
  store ptr %v63, ptr %l23
  %v64 = load ptr, ptr %l23
  ret ptr %v64
b4:
  %v65 = getelementptr %"kg.adt.Result[usize,std/prelude.Error]", ptr %l16, i32 0, i32 1
  %v66 = load i64, ptr %v65
  %v67 = inttoptr i64 %v66 to ptr
  %v68 = call ptr @rt_retain(ptr %v67)
  store ptr %v68, ptr %l18
  call void @"kg.drop.Result[usize,std/prelude.Error]"(ptr %l16)
  %v69 = load ptr, ptr %l2
  store ptr null, ptr %l2
  call void @rt_drop(ptr %v69)
  call void @llvm.memset.p0.i64(ptr %l19, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Result[Void,std/prelude.Error]", ptr null, i32 1) to i64), i1 false)
  %v70 = getelementptr %"kg.adt.Result[Void,std/prelude.Error]", ptr %l19, i32 0, i32 0
  store i32 1, ptr %v70
  %v71 = getelementptr %"kg.adt.Result[Void,std/prelude.Error]", ptr %l19, i32 0, i32 1
  %v72 = load ptr, ptr %l18
  %v73 = ptrtoint ptr %v72 to i64
  store i64 %v73, ptr %v71
  call void @"kg.retain.Result[Void,std/prelude.Error]"(ptr %l19)
  %v74 = call ptr @"kg.box.Result[Void,std/prelude.Error]"(ptr %l19)
  store ptr %v74, ptr %l25
  call void @"kg.drop.Result[Void,std/prelude.Error]"(ptr %l19)
  %v75 = load ptr, ptr %l25
  ret ptr %v75
}
define void @kg_init_statics() {
  ret void
}
define void @kg_fini_statics() {
  ret void
}
define internal void @"kg.drop.Result[usize,std/prelude.Error]"(ptr %v) {
entry:
  %tp = getelementptr %"kg.adt.Result[usize,std/prelude.Error]", ptr %v, i32 0, i32 0
  %pb = getelementptr %"kg.adt.Result[usize,std/prelude.Error]", ptr %v, i32 0, i32 1
  %tag = load i32, ptr %tp
  switch i32 %tag, label %done [ i32 1, label %c1 ]
c1:
  %v1 = load i64, ptr %pb
  %v2 = inttoptr i64 %v1 to ptr
  call void @llvm.memset.p0.i64(ptr %v, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Result[usize,std/prelude.Error]", ptr null, i32 1) to i64), i1 false)
  call void @rt_release(ptr %v2)
  br label %done
done:
  ret void
}
define internal void @"kg.retain.Result[Void,std/prelude.Error]"(ptr %v) {
entry:
  %tp = getelementptr %"kg.adt.Result[Void,std/prelude.Error]", ptr %v, i32 0, i32 0
  %pb = getelementptr %"kg.adt.Result[Void,std/prelude.Error]", ptr %v, i32 0, i32 1
  %tag = load i32, ptr %tp
  switch i32 %tag, label %done [ i32 1, label %c1 ]
c1:
  %v1 = load i64, ptr %pb
  %v2 = inttoptr i64 %v1 to ptr
  call ptr @rt_retain(ptr %v2)
  br label %done
done:
  ret void
}
define internal ptr @"kg.box.Result[Void,std/prelude.Error]"(ptr %v) {
entry:
  %tp = getelementptr %"kg.adt.Result[Void,std/prelude.Error]", ptr %v, i32 0, i32 0
  %pb = getelementptr %"kg.adt.Result[Void,std/prelude.Error]", ptr %v, i32 0, i32 1
  %arr = alloca ptr, i32 1
  %tag = load i32, ptr %tp
  switch i32 %tag, label %bad [ i32 0, label %c0 i32 1, label %c1 ]
c0:
  %v1 = call ptr @rt_unit()
  %v2 = getelementptr ptr, ptr %arr, i32 0
  store ptr %v1, ptr %v2
  %v3 = call ptr @rt_variant(ptr @".variant.universe.Result.Ok", i64 1, ptr %arr)
  ret ptr %v3
c1:
  %v4 = load i64, ptr %pb
  %v5 = inttoptr i64 %v4 to ptr
  %v6 = getelementptr ptr, ptr %arr, i32 0
  store ptr %v5, ptr %v6
  %v7 = call ptr @rt_variant(ptr @".variant.universe.Result.Err", i64 1, ptr %arr)
  ret ptr %v7
bad:
  unreachable
}
define internal void @"kg.drop.Result[Void,std/prelude.Error]"(ptr %v) {
entry:
  %tp = getelementptr %"kg.adt.Result[Void,std/prelude.Error]", ptr %v, i32 0, i32 0
  %pb = getelementptr %"kg.adt.Result[Void,std/prelude.Error]", ptr %v, i32 0, i32 1
  %tag = load i32, ptr %tp
  switch i32 %tag, label %done [ i32 1, label %c1 ]
c1:
  %v1 = load i64, ptr %pb
  %v2 = inttoptr i64 %v1 to ptr
  call void @llvm.memset.p0.i64(ptr %v, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.adt.Result[Void,std/prelude.Error]", ptr null, i32 1) to i64), i1 false)
  call void @rt_release(ptr %v2)
  br label %done
done:
  ret void
}
define i32 @main(i32 %argc, ptr %argv) {
  call void @rt_abi_check(ptr @.p19)
  call void @kg_init_immortals()
  call void @rt_init(i32 %argc, ptr %argv, i32 7, ptr @rt_types)
  call void @kg_init_statics()
  %r = call ptr @"kigumi.main"()
  call void @kg_fini_statics()
  %c = call i32 @rt_exit_code(ptr %r, ptr null, ptr null, i64 0, i64 0)
  ret i32 %c
}

@.p0 = private constant [8 x i8] c".Option\00"
@.p1 = private constant [5 x i8] c"Some\00"
@.p2 = private constant [5 x i8] c"None\00"
@.p3 = private constant [8 x i8] c".Result\00"
@.p4 = private constant [3 x i8] c"Ok\00"
@.p5 = private constant [4 x i8] c"Err\00"
@.p6 = private constant [7 x i8] c".Array\00"
@.p7 = private constant [15 x i8] c"std/ffi.CError\00"
@.p8 = private constant [5 x i8] c"code\00"
@.p9 = private constant [5 x i8] c"text\00"
@.p10 = private constant [12 x i8] c"std/os.Host\00"
@.p11 = private constant [14 x i8] c"std/os.Stdout\00"
@.p12 = private constant [23 x i8] c"std/prelude.AllocError\00"
@.p13 = private constant [12 x i8] c"OutOfMemory\00"
@.p14 = private constant [5 x i8] c"host\00"
@.p15 = internal global [64 x i8] zeroinitializer, align 16
@.p16 = internal global [64 x i8] zeroinitializer, align 16
@.p17 = internal global [64 x i8] zeroinitializer, align 16
@.p18 = private constant [6 x i8] c"print\00"
@.p19 = private constant [40 x i8] c"kgabi1:<hash>\00"
