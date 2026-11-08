%rt_type = type { ptr, i32, i32, ptr, i32, ptr, ptr, i64, i64 }
%rt_variant = type { ptr, ptr, i32, i32, [64 x i8] }
%"kg.agg.golden.Pair" = type { float, float }
declare void @llvm.memset.p0.i64(ptr, i8, i64, i1)
declare void @rt_init(i32, ptr, i32, ptr)
declare void @rt_abi_check(ptr)
declare i32 @rt_exit_code(ptr, ptr, ptr, i64, i64)
declare ptr @rt_unit()
declare ptr @rt_builtin(ptr, i32, ptr)
declare void @rt_const_num(ptr, i32, i32, i64, double)
declare ptr @rt_interp(i32, ptr)
declare void @rt_drop(ptr)
declare void @rt_release(ptr)
declare float @fabsf(float)
declare float @fmaf(float, float, float)
declare float @sqrtf(float)
@".type.universe.Option" = global %rt_type { ptr @.p0, i32 2, i32 0, ptr null, i32 2, ptr @".type.universe.Option.variants", ptr null, i64 0, i64 0 }
@".variant.universe.Option.Some" = global %rt_variant { ptr @.p1, ptr @".type.universe.Option", i32 0, i32 1, [64 x i8] zeroinitializer }
@".variant.universe.Option.None" = global %rt_variant { ptr @.p2, ptr @".type.universe.Option", i32 1, i32 0, [64 x i8] zeroinitializer }
@".type.universe.Option.variants" = global [2 x ptr] [ptr @".variant.universe.Option.Some", ptr @".variant.universe.Option.None"]
@".type.universe.Result" = global %rt_type { ptr @.p3, i32 2, i32 0, ptr null, i32 2, ptr @".type.universe.Result.variants", ptr null, i64 0, i64 0 }
@".variant.universe.Result.Ok" = global %rt_variant { ptr @.p4, ptr @".type.universe.Result", i32 0, i32 1, [64 x i8] zeroinitializer }
@".variant.universe.Result.Err" = global %rt_variant { ptr @.p5, ptr @".type.universe.Result", i32 1, i32 1, [64 x i8] zeroinitializer }
@".type.universe.Result.variants" = global [2 x ptr] [ptr @".variant.universe.Result.Ok", ptr @".variant.universe.Result.Err"]
@".type.golden.Pair" = global %rt_type { ptr @.p6, i32 0, i32 2, ptr @".type.golden.Pair.fields", i32 0, ptr null, ptr null, i64 0, i64 0 }
@".type.golden.Pair.fields" = global [2 x ptr] [ptr @.p7, ptr @.p8]
@rt_types = global [3 x ptr] [ptr @".type.universe.Option", ptr @".type.universe.Result", ptr @".type.golden.Pair"]
define float @"golden.hyp"(%"kg.agg.golden.Pair" %p0) {
entry:
  %l0 = alloca %"kg.agg.golden.Pair"
  call void @llvm.memset.p0.i64(ptr %l0, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.agg.golden.Pair", ptr null, i32 1) to i64), i1 false)
  %l1 = alloca float
  store float 0.0, ptr %l1
  %l2 = alloca float
  store float 0.0, ptr %l2
  %l3 = alloca float
  store float 0.0, ptr %l3
  %l4 = alloca float
  store float 0.0, ptr %l4
  %l5 = alloca float
  store float 0.0, ptr %l5
  %l6 = alloca float
  store float 0.0, ptr %l6
  %l7 = alloca float
  store float 0.0, ptr %l7
  %l8 = alloca float
  store float 0.0, ptr %l8
  store %"kg.agg.golden.Pair" %p0, ptr %l0
  br label %b0
b0:
  %v1 = getelementptr %"kg.agg.golden.Pair", ptr %l0, i32 0, i32 0
  %v2 = load float, ptr %v1
  store float %v2, ptr %l1
  %v3 = getelementptr %"kg.agg.golden.Pair", ptr %l0, i32 0, i32 0
  %v4 = load float, ptr %v3
  store float %v4, ptr %l2
  %v5 = load float, ptr %l1
  %v6 = load float, ptr %l2
  %v7 = fmul float %v5, %v6
  store float %v7, ptr %l3
  %v8 = getelementptr %"kg.agg.golden.Pair", ptr %l0, i32 0, i32 1
  %v9 = load float, ptr %v8
  store float %v9, ptr %l4
  %v10 = getelementptr %"kg.agg.golden.Pair", ptr %l0, i32 0, i32 1
  %v11 = load float, ptr %v10
  store float %v11, ptr %l5
  %v12 = load float, ptr %l4
  %v13 = load float, ptr %l5
  %v14 = fmul float %v12, %v13
  store float %v14, ptr %l6
  %v15 = load float, ptr %l3
  %v16 = load float, ptr %l6
  %v17 = fadd float %v15, %v16
  store float %v17, ptr %l7
  %v18 = load float, ptr %l7
  %v19 = call float (float) @sqrtf(float %v18)
  store float %v19, ptr %l8
  %v20 = load float, ptr %l8
  ret float %v20
}
define ptr @"kigumi.main"() {
entry:
  %l0 = alloca float
  store float 0.0, ptr %l0
  %l1 = alloca float
  store float 0.0, ptr %l1
  %l2 = alloca float
  store float 0.0, ptr %l2
  %l3 = alloca float
  store float 0.0, ptr %l3
  %l4 = alloca ptr
  store ptr null, ptr %l4
  %l5 = alloca float
  store float 0.0, ptr %l5
  %l6 = alloca float
  store float 0.0, ptr %l6
  %l7 = alloca float
  store float 0.0, ptr %l7
  %l8 = alloca float
  store float 0.0, ptr %l8
  %l9 = alloca float
  store float 0.0, ptr %l9
  %l10 = alloca float
  store float 0.0, ptr %l10
  %l11 = alloca float
  store float 0.0, ptr %l11
  %l12 = alloca float
  store float 0.0, ptr %l12
  %l13 = alloca ptr
  store ptr null, ptr %l13
  %l14 = alloca ptr
  store ptr null, ptr %l14
  %l15 = alloca float
  store float 0.0, ptr %l15
  %l16 = alloca float
  store float 0.0, ptr %l16
  %l17 = alloca %"kg.agg.golden.Pair"
  call void @llvm.memset.p0.i64(ptr %l17, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.agg.golden.Pair", ptr null, i32 1) to i64), i1 false)
  %l18 = alloca float
  store float 0.0, ptr %l18
  %l19 = alloca float
  store float 0.0, ptr %l19
  %l20 = alloca float
  store float 0.0, ptr %l20
  %l21 = alloca %"kg.agg.golden.Pair"
  call void @llvm.memset.p0.i64(ptr %l21, i8 0, i64 ptrtoint (ptr getelementptr (%"kg.agg.golden.Pair", ptr null, i32 1) to i64), i1 false)
  %l22 = alloca float
  store float 0.0, ptr %l22
  %l23 = alloca ptr
  store ptr null, ptr %l23
  %l24 = alloca ptr
  store ptr null, ptr %l24
  %l25 = alloca float
  store float 0.0, ptr %l25
  %l26 = alloca float
  store float 0.0, ptr %l26
  %v1 = alloca [64 x i8], align 16
  %v2 = alloca [64 x i8], align 16
  %v3 = alloca [64 x i8], align 16
  %v4 = alloca ptr, i32 5
  %v5 = alloca ptr, i32 1
  %v6 = alloca [64 x i8], align 16
  %v7 = alloca [64 x i8], align 16
  %v8 = alloca ptr, i32 3
  %v9 = alloca ptr, i32 1
  br label %b0
b0:
  store float 0x3FB99999A0000000, ptr %l0
  %v10 = load float, ptr %l0
  store float %v10, ptr %l1
  store float 0x3FE6666660000000, ptr %l2
  %v11 = load float, ptr %l2
  store float %v11, ptr %l3
  store float 0x4000000000000000, ptr %l5
  %v12 = load float, ptr %l5
  %v13 = call float (float) @sqrtf(float %v12)
  store float %v13, ptr %l6
  %v14 = load float, ptr %l1
  %v15 = fneg float %v14
  store float %v15, ptr %l7
  %v16 = load float, ptr %l7
  %v17 = call float (float) @fabsf(float %v16)
  store float %v17, ptr %l8
  %v18 = load float, ptr %l1
  store float %v18, ptr %l9
  %v19 = load float, ptr %l3
  store float %v19, ptr %l10
  store float 0x3FF0000000000000, ptr %l11
  %v20 = load float, ptr %l9
  %v21 = load float, ptr %l10
  %v22 = load float, ptr %l11
  %v23 = call float (float, float, float) @fmaf(float %v20, float %v21, float %v22)
  store float %v23, ptr %l12
  %v24 = load float, ptr %l6
  %v25 = fpext float %v24 to double
  call void @rt_const_num(ptr %v1, i32 2, i32 544, i64 0, double %v25)
  %v26 = load float, ptr %l8
  %v27 = fpext float %v26 to double
  call void @rt_const_num(ptr %v2, i32 2, i32 544, i64 0, double %v27)
  %v28 = load float, ptr %l12
  %v29 = fpext float %v28 to double
  call void @rt_const_num(ptr %v3, i32 2, i32 544, i64 0, double %v29)
  %v30 = getelementptr ptr, ptr %v4, i32 0
  store ptr %v1, ptr %v30
  %v31 = getelementptr ptr, ptr %v4, i32 1
  store ptr @.p9, ptr %v31
  %v32 = getelementptr ptr, ptr %v4, i32 2
  store ptr %v2, ptr %v32
  %v33 = getelementptr ptr, ptr %v4, i32 3
  store ptr @.p9, ptr %v33
  %v34 = getelementptr ptr, ptr %v4, i32 4
  store ptr %v3, ptr %v34
  %v35 = call ptr @rt_interp(i32 5, ptr %v4)
  call void @rt_release(ptr %v1)
  call void @rt_release(ptr %v2)
  call void @rt_release(ptr %v3)
  store ptr %v35, ptr %l4
  %v36 = load ptr, ptr %l4
  %v37 = getelementptr ptr, ptr %v5, i32 0
  store ptr %v36, ptr %v37
  %v38 = call ptr @rt_builtin(ptr @.p10, i32 1, ptr %v5)
  store ptr %v38, ptr %l13
  %v39 = load ptr, ptr %l4
  call void @rt_release(ptr %v39)
  %v40 = load ptr, ptr %l13
  call void @rt_drop(ptr %v40)
  store float 0x4008000000000000, ptr %l15
  store float 0x4010000000000000, ptr %l16
  %v41 = getelementptr %"kg.agg.golden.Pair", ptr %l17, i32 0, i32 0
  %v42 = load float, ptr %l15
  store float %v42, ptr %v41
  %v43 = getelementptr %"kg.agg.golden.Pair", ptr %l17, i32 0, i32 1
  %v44 = load float, ptr %l16
  store float %v44, ptr %v43
  %v45 = load %"kg.agg.golden.Pair", ptr %l17
  %v46 = call float @"golden.hyp"(%"kg.agg.golden.Pair" %v45)
  store float %v46, ptr %l18
  %v47 = load float, ptr %l1
  store float %v47, ptr %l19
  %v48 = load float, ptr %l3
  store float %v48, ptr %l20
  %v49 = load float, ptr %l19
  store float %v49, ptr %l25
  %v50 = load float, ptr %l20
  store float %v50, ptr %l26
  %v51 = getelementptr %"kg.agg.golden.Pair", ptr %l21, i32 0, i32 0
  %v52 = load float, ptr %l25
  store float %v52, ptr %v51
  %v53 = getelementptr %"kg.agg.golden.Pair", ptr %l21, i32 0, i32 1
  %v54 = load float, ptr %l26
  store float %v54, ptr %v53
  %v55 = load %"kg.agg.golden.Pair", ptr %l21
  %v56 = call float @"golden.hyp"(%"kg.agg.golden.Pair" %v55)
  store float %v56, ptr %l22
  %v57 = load float, ptr %l18
  %v58 = fpext float %v57 to double
  call void @rt_const_num(ptr %v6, i32 2, i32 544, i64 0, double %v58)
  %v59 = load float, ptr %l22
  %v60 = fpext float %v59 to double
  call void @rt_const_num(ptr %v7, i32 2, i32 544, i64 0, double %v60)
  %v61 = getelementptr ptr, ptr %v8, i32 0
  store ptr %v6, ptr %v61
  %v62 = getelementptr ptr, ptr %v8, i32 1
  store ptr @.p9, ptr %v62
  %v63 = getelementptr ptr, ptr %v8, i32 2
  store ptr %v7, ptr %v63
  %v64 = call ptr @rt_interp(i32 3, ptr %v8)
  call void @rt_release(ptr %v6)
  call void @rt_release(ptr %v7)
  store ptr %v64, ptr %l14
  %v65 = load ptr, ptr %l14
  %v66 = getelementptr ptr, ptr %v9, i32 0
  store ptr %v65, ptr %v66
  %v67 = call ptr @rt_builtin(ptr @.p10, i32 1, ptr %v9)
  store ptr %v67, ptr %l23
  %v68 = load ptr, ptr %l14
  call void @rt_release(ptr %v68)
  %v69 = load ptr, ptr %l23
  call void @rt_drop(ptr %v69)
  %v70 = call ptr @rt_unit()
  store ptr %v70, ptr %l24
  %v71 = load ptr, ptr %l24
  ret ptr %v71
}
define void @kg_init_statics() {
  ret void
}
define void @kg_fini_statics() {
  ret void
}
define i32 @main(i32 %argc, ptr %argv) {
  call void @rt_abi_check(ptr @.p11)
  call void @kg_init_immortals()
  call void @rt_init(i32 %argc, ptr %argv, i32 3, ptr @rt_types)
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
@.p6 = private constant [12 x i8] c"golden.Pair\00"
@.p7 = private constant [2 x i8] c"a\00"
@.p8 = private constant [2 x i8] c"b\00"
@.p9 = internal global [64 x i8] zeroinitializer, align 16
@.p10 = private constant [6 x i8] c"print\00"
@.p11 = private constant [40 x i8] c"kgabi1:<hash>\00"
