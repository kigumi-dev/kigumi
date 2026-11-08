%rt_type = type { ptr, i32, i32, ptr, i32, ptr, ptr, i64, i64 }
%rt_variant = type { ptr, ptr, i32, i32, [64 x i8] }
@kg_cancel_marker = external global i8
declare void @llvm.memset.p0.i64(ptr, i8, i64, i1)
declare { i64, i1 } @llvm.sadd.with.overflow.i64(i64, i64)
declare { i64, i1 } @llvm.smul.with.overflow.i64(i64, i64)
declare void @rt_init(i32, ptr, i32, ptr)
declare void @rt_abi_check(ptr)
declare i32 @rt_exit_code(ptr, ptr, ptr, i64, i64)
declare ptr @rt_unit()
declare ptr @rt_int(i64, i32)
declare ptr @rt_bool(i32)
declare i1 @rt_truth(ptr)
declare ptr @rt_builtin(ptr, i32, ptr)
declare ptr @rt_std_id(i32, i32, ptr)
declare ptr @rt_interp(i32, ptr)
declare void @rt_drop(ptr)
declare void @rt_release(ptr)
declare void @rt_panic(ptr)
declare i64 @rt_int_val(ptr)
declare ptr @rt_future_new(ptr, i32, ptr)
declare ptr @rt_future_slots(ptr)
declare i32 @rt_future_state(ptr)
declare ptr @rt_future_arg(ptr, i64)
declare void @rt_future_started(ptr)
declare void @rt_future_suspend(ptr, i32, ptr)
declare ptr @rt_future_resume(ptr)
declare ptr @rt_future_complete(ptr, ptr)
declare ptr @rt_task_close(ptr)
declare void @rt_unwind_enter(ptr, i64)
declare void @rt_unwind_enter_slots(ptr, ptr, ptr, i64)
declare void @rt_unwind_leave(ptr)
declare void @rt_drop_outer(ptr)
declare i32 @rt_release_outer(ptr)
declare void @rt_sweep_release(ptr)
declare void @rt_outer_fatal()
@".type.universe.Option" = global %rt_type { ptr @.p0, i32 2, i32 0, ptr null, i32 2, ptr @".type.universe.Option.variants", ptr null, i64 0, i64 0 }
@".variant.universe.Option.Some" = global %rt_variant { ptr @.p1, ptr @".type.universe.Option", i32 0, i32 1, [64 x i8] zeroinitializer }
@".variant.universe.Option.None" = global %rt_variant { ptr @.p2, ptr @".type.universe.Option", i32 1, i32 0, [64 x i8] zeroinitializer }
@".type.universe.Option.variants" = global [2 x ptr] [ptr @".variant.universe.Option.Some", ptr @".variant.universe.Option.None"]
@".type.universe.Result" = global %rt_type { ptr @.p3, i32 2, i32 0, ptr null, i32 2, ptr @".type.universe.Result.variants", ptr null, i64 0, i64 0 }
@".variant.universe.Result.Ok" = global %rt_variant { ptr @.p4, ptr @".type.universe.Result", i32 0, i32 1, [64 x i8] zeroinitializer }
@".variant.universe.Result.Err" = global %rt_variant { ptr @.p5, ptr @".type.universe.Result", i32 1, i32 1, [64 x i8] zeroinitializer }
@".type.universe.Result.variants" = global [2 x ptr] [ptr @".variant.universe.Result.Ok", ptr @".variant.universe.Result.Err"]
@".type.universe.Future" = global %rt_type { ptr @.p6, i32 0, i32 0, ptr null, i32 0, ptr null, ptr null, i64 0, i64 0 }
@".type.std/task.Executor" = global %rt_type { ptr @.p7, i32 0, i32 0, ptr null, i32 0, ptr null, ptr null, i64 0, i64 0 }
@".type.std/task.Task" = global %rt_type { ptr @.p8, i32 0, i32 0, ptr null, i32 0, ptr null, ptr null, i64 0, i64 0 }
@rt_types = global [5 x ptr] [ptr @".type.universe.Option", ptr @".type.universe.Result", ptr @".type.universe.Future", ptr @".type.std/task.Executor", ptr @".type.std/task.Task"]
@"golden.root$async.fnitem.async" = internal global [64 x i8] zeroinitializer, align 16
@"golden.total$async.fnitem.async" = internal global [64 x i8] zeroinitializer, align 16
@"golden.twice$async.fnitem.async" = internal global [64 x i8] zeroinitializer, align 16
define ptr @"golden.twice"(ptr %fut) {
entry:
  %slots = call ptr @rt_future_slots(ptr %fut)
  %urec = alloca [6 x i64], align 8
  %umask = alloca [1 x i64]
  call void @llvm.memset.p0.i64(ptr %umask, i8 0, i64 8, i1 false)
  %l0 = getelementptr ptr, ptr %slots, i64 0
  %l1 = getelementptr ptr, ptr %slots, i64 1
  %l2 = alloca ptr
  store ptr null, ptr %l2
  %l3 = alloca i64
  store i64 0, ptr %l3
  %l4 = alloca ptr
  store ptr null, ptr %l4
  %l5 = alloca ptr
  store ptr null, ptr %l5
  %v1 = alloca ptr, i32 2
  %v2 = alloca ptr, i32 1
  call void @rt_unwind_enter_slots(ptr %urec, ptr %slots, ptr %umask, i64 2)
  %st = call i32 @rt_future_state(ptr %fut)
  switch i32 %st, label %resume.bad [ i32 0, label %start ]
resume.bad:
  call void @rt_panic(ptr @.p9)
  unreachable
start:
  %v3 = call ptr @rt_future_arg(ptr %fut, i64 0)
  store ptr %v3, ptr %l0
  call void @rt_future_started(ptr %fut)
  br label %b0
b0:
  %v4 = getelementptr i64, ptr %umask, i64 0
  store i64 0, ptr %v4
  %v5 = load ptr, ptr %l0
  %v6 = getelementptr ptr, ptr %v1, i32 0
  store ptr @.p10, ptr %v6
  %v7 = getelementptr ptr, ptr %v1, i32 1
  store ptr %v5, ptr %v7
  %v8 = call ptr @rt_interp(i32 2, ptr %v1)
  store ptr %v8, ptr %l1
  %v9 = getelementptr i64, ptr %umask, i64 0
  store i64 2, ptr %v9
  %v10 = load ptr, ptr %l1
  %v11 = getelementptr ptr, ptr %v2, i32 0
  store ptr %v10, ptr %v11
  %v12 = call ptr @rt_builtin(ptr @.p11, i32 1, ptr %v2)
  store ptr %v12, ptr %l2
  %v13 = getelementptr i64, ptr %umask, i64 0
  store i64 0, ptr %v13
  %v14 = load ptr, ptr %l1
  call void @rt_release(ptr %v14)
  %v15 = load ptr, ptr %l2
  call void @rt_drop(ptr %v15)
  store i64 2, ptr %l3
  %v16 = load ptr, ptr %l0
  %v17 = call i64 @rt_int_val(ptr %v16)
  %v18 = load i64, ptr %l3
  %v19 = call { i64, i1 } @llvm.smul.with.overflow.i64(i64 %v17, i64 %v18)
  %v20 = extractvalue { i64, i1 } %v19, 0
  %v21 = extractvalue { i64, i1 } %v19, 1
  br i1 %v21, label %ovf21, label %ovfok22
ovf21:
  call void @rt_panic(ptr @.p12)
  unreachable
ovfok22:
  %v22 = call ptr @rt_int(i64 %v20, i32 320)
  store ptr %v22, ptr %l4
  %v23 = call ptr @rt_task_close(ptr %fut)
  store ptr %v23, ptr %l5
  %v24 = load ptr, ptr %l0
  store ptr null, ptr %l0
  call void @rt_drop(ptr %v24)
  %v25 = load ptr, ptr %l4
  %v26 = call ptr @rt_future_complete(ptr %fut, ptr %v25)
  call void @rt_unwind_leave(ptr %urec)
  %v27 = load ptr, ptr %l4
  ret ptr %v27
}
define internal void @"golden.twice$framedrop"(ptr %fut) {
entry:
  %slots = call ptr @rt_future_slots(ptr %fut)
  %st = call i32 @rt_future_state(ptr %fut)
  switch i32 %st, label %done [ i32 1073741824, label %dp ]
dp:
  %v1 = getelementptr ptr, ptr %slots, i64 0
  %v2 = load ptr, ptr %v1
  store ptr null, ptr %v1
  call void @rt_release(ptr %v2)
  br label %done
done:
  ret void
}
define ptr @"golden.total"(ptr %fut) {
entry:
  %slots = call ptr @rt_future_slots(ptr %fut)
  %urec = alloca [6 x i64], align 8
  %umask = alloca [1 x i64]
  call void @llvm.memset.p0.i64(ptr %umask, i8 0, i64 8, i1 false)
  %l0 = getelementptr ptr, ptr %slots, i64 0
  %l1 = getelementptr ptr, ptr %slots, i64 1
  %l2 = alloca ptr
  store ptr null, ptr %l2
  %l3 = getelementptr ptr, ptr %slots, i64 2
  %l4 = getelementptr ptr, ptr %slots, i64 3
  %l5 = getelementptr ptr, ptr %slots, i64 4
  %l6 = alloca ptr
  store ptr null, ptr %l6
  %l7 = getelementptr ptr, ptr %slots, i64 5
  %l8 = alloca ptr
  store ptr null, ptr %l8
  %l9 = getelementptr ptr, ptr %slots, i64 6
  %l10 = getelementptr ptr, ptr %slots, i64 7
  %l11 = getelementptr ptr, ptr %slots, i64 8
  %l12 = alloca ptr
  store ptr null, ptr %l12
  %l13 = getelementptr ptr, ptr %slots, i64 9
  %l14 = alloca ptr
  store ptr null, ptr %l14
  %l15 = alloca ptr
  store ptr null, ptr %l15
  %v1 = alloca ptr, i32 1
  %v2 = alloca ptr, i32 1
  %v3 = alloca ptr, i32 1
  %v4 = alloca ptr, i32 1
  call void @rt_unwind_enter_slots(ptr %urec, ptr %slots, ptr %umask, i64 10)
  %st = call i32 @rt_future_state(ptr %fut)
  switch i32 %st, label %resume.bad [ i32 0, label %start i32 1, label %rs1 i32 2, label %rs2 ]
resume.bad:
  call void @rt_panic(ptr @.p9)
  unreachable
start:
  %v5 = call ptr @rt_future_arg(ptr %fut, i64 0)
  store ptr %v5, ptr %l0
  %v6 = call ptr @rt_future_arg(ptr %fut, i64 1)
  store ptr %v6, ptr %l1
  call void @rt_future_started(ptr %fut)
  br label %b0
b0:
  %v7 = load ptr, ptr %l0
  store ptr %v7, ptr %l2
  store ptr @"golden.twice$async.fnitem.async", ptr %l3
  %v8 = getelementptr i64, ptr %umask, i64 0
  store i64 4, ptr %v8
  %v9 = load ptr, ptr %l3
  %v10 = load ptr, ptr %l2
  %v11 = getelementptr ptr, ptr %v2, i32 0
  store ptr %v10, ptr %v11
  %v12 = call ptr @rt_future_new(ptr %v9, i32 1, ptr %v2)
  store ptr %v12, ptr %l4
  %v13 = getelementptr i64, ptr %umask, i64 0
  store i64 8, ptr %v13
  %v14 = load ptr, ptr %l3
  call void @rt_release(ptr %v14)
  %v15 = getelementptr i64, ptr %umask, i64 0
  store i64 0, ptr %v15
  %v16 = load ptr, ptr %l4
  call void @rt_future_suspend(ptr %fut, i32 1, ptr %v16)
  br label %rs1
rs1:
  %v17 = getelementptr i64, ptr %umask, i64 0
  store i64 0, ptr %v17
  %v18 = call ptr @rt_future_resume(ptr %fut)
  %v19 = icmp eq ptr %v18, null
  br i1 %v19, label %pend1, label %ck1
pend1:
  call void @rt_unwind_leave(ptr %urec)
  ret ptr null
ck1:
  %v20 = icmp eq ptr %v18, @kg_cancel_marker
  br i1 %v20, label %can1, label %cont1
can1:
  store ptr null, ptr %l5
  br label %join1
cont1:
  store ptr %v18, ptr %l5
  br label %join1
join1:
  %cf1 = phi i1 [ true, %can1 ], [ false, %cont1 ]
  %v21 = zext i1 %cf1 to i32
  %v22 = call ptr @rt_bool(i32 %v21)
  store ptr %v22, ptr %l6
  %v23 = load ptr, ptr %l6
  %v24 = call i1 @rt_truth(ptr %v23)
  br i1 %v24, label %b5, label %b2
b1:
  %v25 = getelementptr i64, ptr %umask, i64 0
  store i64 0, ptr %v25
  %v26 = load ptr, ptr %l1
  store ptr null, ptr %l1
  call void @rt_drop(ptr %v26)
  %v27 = load ptr, ptr %l0
  store ptr null, ptr %l0
  call void @rt_drop(ptr %v27)
  %v28 = call ptr @rt_future_complete(ptr %fut, ptr null)
  call void @rt_unwind_leave(ptr %urec)
  ret ptr @kg_cancel_marker
b2:
  %v29 = load ptr, ptr %l5
  %v30 = call i64 @rt_int_val(ptr %v29)
  call void @rt_release(ptr %v29)
  store i64 %v30, ptr %l7
  %v31 = load ptr, ptr %l1
  store ptr %v31, ptr %l8
  store ptr @"golden.twice$async.fnitem.async", ptr %l9
  %v32 = getelementptr i64, ptr %umask, i64 0
  store i64 64, ptr %v32
  %v33 = load ptr, ptr %l9
  %v34 = load ptr, ptr %l8
  %v35 = getelementptr ptr, ptr %v4, i32 0
  store ptr %v34, ptr %v35
  %v36 = call ptr @rt_future_new(ptr %v33, i32 1, ptr %v4)
  store ptr %v36, ptr %l10
  %v37 = getelementptr i64, ptr %umask, i64 0
  store i64 128, ptr %v37
  %v38 = load ptr, ptr %l9
  call void @rt_release(ptr %v38)
  %v39 = getelementptr i64, ptr %umask, i64 0
  store i64 0, ptr %v39
  %v40 = load ptr, ptr %l10
  call void @rt_future_suspend(ptr %fut, i32 2, ptr %v40)
  br label %rs2
rs2:
  %v41 = getelementptr i64, ptr %umask, i64 0
  store i64 0, ptr %v41
  %v42 = call ptr @rt_future_resume(ptr %fut)
  %v43 = icmp eq ptr %v42, null
  br i1 %v43, label %pend2, label %ck2
pend2:
  call void @rt_unwind_leave(ptr %urec)
  ret ptr null
ck2:
  %v44 = icmp eq ptr %v42, @kg_cancel_marker
  br i1 %v44, label %can2, label %cont2
can2:
  store ptr null, ptr %l11
  br label %join2
cont2:
  store ptr %v42, ptr %l11
  br label %join2
join2:
  %cf2 = phi i1 [ true, %can2 ], [ false, %cont2 ]
  %v45 = zext i1 %cf2 to i32
  %v46 = call ptr @rt_bool(i32 %v45)
  store ptr %v46, ptr %l12
  %v47 = load ptr, ptr %l12
  %v48 = call i1 @rt_truth(ptr %v47)
  br i1 %v48, label %b3, label %b4
b3:
  %v49 = getelementptr i64, ptr %umask, i64 0
  store i64 0, ptr %v49
  %v50 = load ptr, ptr %l11
  call void @rt_release(ptr %v50)
  br label %b1
b4:
  %v51 = load ptr, ptr %l11
  %v52 = call i64 @rt_int_val(ptr %v51)
  call void @rt_release(ptr %v51)
  store i64 %v52, ptr %l13
  %v53 = getelementptr i64, ptr %umask, i64 0
  store i64 0, ptr %v53
  %v54 = load i64, ptr %l7
  %v55 = load i64, ptr %l13
  %v56 = call { i64, i1 } @llvm.sadd.with.overflow.i64(i64 %v54, i64 %v55)
  %v57 = extractvalue { i64, i1 } %v56, 0
  %v58 = extractvalue { i64, i1 } %v56, 1
  br i1 %v58, label %ovf57, label %ovfok58
ovf57:
  call void @rt_panic(ptr @.p12)
  unreachable
ovfok58:
  %v59 = call ptr @rt_int(i64 %v57, i32 320)
  store ptr %v59, ptr %l14
  %v60 = call ptr @rt_task_close(ptr %fut)
  store ptr %v60, ptr %l15
  %v61 = load ptr, ptr %l1
  store ptr null, ptr %l1
  call void @rt_drop(ptr %v61)
  %v62 = load ptr, ptr %l0
  store ptr null, ptr %l0
  call void @rt_drop(ptr %v62)
  %v63 = load ptr, ptr %l14
  %v64 = call ptr @rt_future_complete(ptr %fut, ptr %v63)
  call void @rt_unwind_leave(ptr %urec)
  %v65 = load ptr, ptr %l14
  ret ptr %v65
b5:
  %v66 = getelementptr i64, ptr %umask, i64 0
  store i64 0, ptr %v66
  %v67 = load ptr, ptr %l5
  call void @rt_release(ptr %v67)
  br label %b1
}
define internal void @"golden.total$framedrop"(ptr %fut) {
entry:
  %slots = call ptr @rt_future_slots(ptr %fut)
  %st = call i32 @rt_future_state(ptr %fut)
  switch i32 %st, label %done [ i32 1, label %d1 i32 2, label %d2 i32 1073741824, label %dp ]
d1:
  %v1 = getelementptr ptr, ptr %slots, i64 4
  %v2 = load ptr, ptr %v1
  store ptr null, ptr %v1
  call void @rt_release(ptr %v2)
  %v3 = getelementptr ptr, ptr %slots, i64 1
  %v4 = load ptr, ptr %v3
  store ptr null, ptr %v3
  call void @rt_release(ptr %v4)
  %v5 = getelementptr ptr, ptr %slots, i64 0
  %v6 = load ptr, ptr %v5
  store ptr null, ptr %v5
  call void @rt_release(ptr %v6)
  br label %done
d2:
  %v7 = getelementptr ptr, ptr %slots, i64 8
  %v8 = load ptr, ptr %v7
  store ptr null, ptr %v7
  call void @rt_release(ptr %v8)
  %v9 = getelementptr ptr, ptr %slots, i64 1
  %v10 = load ptr, ptr %v9
  store ptr null, ptr %v9
  call void @rt_release(ptr %v10)
  %v11 = getelementptr ptr, ptr %slots, i64 0
  %v12 = load ptr, ptr %v11
  store ptr null, ptr %v11
  call void @rt_release(ptr %v12)
  br label %done
dp:
  %v13 = getelementptr ptr, ptr %slots, i64 1
  %v14 = load ptr, ptr %v13
  store ptr null, ptr %v13
  call void @rt_release(ptr %v14)
  %v15 = getelementptr ptr, ptr %slots, i64 0
  %v16 = load ptr, ptr %v15
  store ptr null, ptr %v15
  call void @rt_release(ptr %v16)
  br label %done
done:
  ret void
}
define ptr @"golden.root"(ptr %fut) {
entry:
  %slots = call ptr @rt_future_slots(ptr %fut)
  %urec = alloca [6 x i64], align 8
  %umask = alloca [1 x i64]
  call void @llvm.memset.p0.i64(ptr %umask, i8 0, i64 8, i1 false)
  %l0 = alloca ptr
  store ptr null, ptr %l0
  %l1 = alloca ptr
  store ptr null, ptr %l1
  %l2 = getelementptr ptr, ptr %slots, i64 0
  %l3 = alloca ptr
  store ptr null, ptr %l3
  %l4 = getelementptr ptr, ptr %slots, i64 1
  %l5 = alloca ptr
  store ptr null, ptr %l5
  %l6 = alloca ptr
  store ptr null, ptr %l6
  %l7 = alloca ptr
  store ptr null, ptr %l7
  %l8 = getelementptr ptr, ptr %slots, i64 2
  %l9 = getelementptr ptr, ptr %slots, i64 3
  %l10 = alloca ptr
  store ptr null, ptr %l10
  %l11 = getelementptr ptr, ptr %slots, i64 4
  %l12 = alloca ptr
  store ptr null, ptr %l12
  %l13 = getelementptr ptr, ptr %slots, i64 5
  %l14 = alloca ptr
  store ptr null, ptr %l14
  %l15 = getelementptr ptr, ptr %slots, i64 6
  %l16 = alloca ptr
  store ptr null, ptr %l16
  %l17 = getelementptr ptr, ptr %slots, i64 7
  %l18 = alloca ptr
  store ptr null, ptr %l18
  %l19 = getelementptr ptr, ptr %slots, i64 8
  %l20 = getelementptr ptr, ptr %slots, i64 9
  %l21 = alloca ptr
  store ptr null, ptr %l21
  %l22 = alloca ptr
  store ptr null, ptr %l22
  %l23 = alloca ptr
  store ptr null, ptr %l23
  %v1 = alloca ptr, i32 1
  %v2 = alloca ptr, i32 2
  %v3 = alloca ptr, i32 1
  %v4 = alloca ptr, i32 1
  %v5 = alloca ptr, i32 1
  %v6 = alloca ptr, i32 1
  %v7 = alloca ptr, i32 3
  %v8 = alloca ptr, i32 1
  call void @rt_unwind_enter_slots(ptr %urec, ptr %slots, ptr %umask, i64 10)
  %st = call i32 @rt_future_state(ptr %fut)
  switch i32 %st, label %resume.bad [ i32 0, label %start i32 1, label %rs1 i32 2, label %rs2 ]
resume.bad:
  call void @rt_panic(ptr @.p9)
  unreachable
start:
  call void @rt_future_started(ptr %fut)
  br label %b0
b0:
  store ptr @.p13, ptr %l0
  store ptr @.p14, ptr %l1
  store ptr @"golden.total$async.fnitem.async", ptr %l2
  %v9 = getelementptr i64, ptr %umask, i64 0
  store i64 1, ptr %v9
  %v10 = load ptr, ptr %l2
  %v11 = load ptr, ptr %l0
  %v12 = load ptr, ptr %l1
  %v13 = getelementptr ptr, ptr %v2, i32 0
  store ptr %v11, ptr %v13
  %v14 = getelementptr ptr, ptr %v2, i32 1
  store ptr %v12, ptr %v14
  %v15 = call ptr @rt_future_new(ptr %v10, i32 2, ptr %v2)
  store ptr %v15, ptr %l3
  %v16 = load ptr, ptr %l0
  call void @rt_release(ptr %v16)
  %v17 = load ptr, ptr %l1
  call void @rt_release(ptr %v17)
  %v18 = getelementptr i64, ptr %umask, i64 0
  store i64 0, ptr %v18
  %v19 = load ptr, ptr %l2
  call void @rt_release(ptr %v19)
  %v20 = load ptr, ptr %l3
  store ptr %v20, ptr %l4
  store ptr @.p15, ptr %l5
  %v21 = load ptr, ptr %l5
  %v22 = getelementptr ptr, ptr %v3, i32 0
  store ptr %v21, ptr %v22
  %v23 = call ptr @rt_builtin(ptr @.p11, i32 1, ptr %v3)
  store ptr %v23, ptr %l6
  %v24 = load ptr, ptr %l5
  call void @rt_release(ptr %v24)
  %v25 = load ptr, ptr %l6
  call void @rt_drop(ptr %v25)
  store ptr @.p16, ptr %l7
  store ptr @"golden.twice$async.fnitem.async", ptr %l8
  %v26 = getelementptr i64, ptr %umask, i64 0
  store i64 4, ptr %v26
  %v27 = load ptr, ptr %l8
  %v28 = load ptr, ptr %l7
  %v29 = getelementptr ptr, ptr %v5, i32 0
  store ptr %v28, ptr %v29
  %v30 = call ptr @rt_future_new(ptr %v27, i32 1, ptr %v5)
  store ptr %v30, ptr %l9
  %v31 = getelementptr i64, ptr %umask, i64 0
  store i64 12, ptr %v31
  %v32 = load ptr, ptr %l7
  call void @rt_release(ptr %v32)
  %v33 = getelementptr i64, ptr %umask, i64 0
  store i64 8, ptr %v33
  %v34 = load ptr, ptr %l8
  call void @rt_release(ptr %v34)
  %v35 = load ptr, ptr %l9
  %v36 = getelementptr ptr, ptr %v6, i32 0
  store ptr %v35, ptr %v36
  %v37 = call ptr @rt_std_id(i32 55, i32 1, ptr %v6)
  store ptr %v37, ptr %l10
  %v38 = getelementptr i64, ptr %umask, i64 0
  store i64 0, ptr %v38
  %v39 = load ptr, ptr %l9
  call void @rt_release(ptr %v39)
  %v40 = load ptr, ptr %l10
  store ptr %v40, ptr %l11
  %v41 = load ptr, ptr %l4
  store ptr null, ptr %l4
  store ptr %v41, ptr %l12
  %v42 = load ptr, ptr %l12
  call void @rt_future_suspend(ptr %fut, i32 1, ptr %v42)
  br label %rs1
rs1:
  %v43 = getelementptr i64, ptr %umask, i64 0
  store i64 0, ptr %v43
  %v44 = call ptr @rt_future_resume(ptr %fut)
  %v45 = icmp eq ptr %v44, null
  br i1 %v45, label %pend1, label %ck1
pend1:
  call void @rt_unwind_leave(ptr %urec)
  ret ptr null
ck1:
  %v46 = icmp eq ptr %v44, @kg_cancel_marker
  br i1 %v46, label %can1, label %cont1
can1:
  store ptr null, ptr %l13
  br label %join1
cont1:
  store ptr %v44, ptr %l13
  br label %join1
join1:
  %cf1 = phi i1 [ true, %can1 ], [ false, %cont1 ]
  %v47 = zext i1 %cf1 to i32
  %v48 = call ptr @rt_bool(i32 %v47)
  store ptr %v48, ptr %l14
  %v49 = load ptr, ptr %l14
  %v50 = call i1 @rt_truth(ptr %v49)
  br i1 %v50, label %b5, label %b2
b1:
  %v51 = getelementptr i64, ptr %umask, i64 0
  store i64 0, ptr %v51
  %v52 = load ptr, ptr %l11
  store ptr null, ptr %l11
  call void @rt_drop(ptr %v52)
  %v53 = call ptr @rt_future_complete(ptr %fut, ptr null)
  call void @rt_unwind_leave(ptr %urec)
  ret ptr @kg_cancel_marker
b2:
  %v54 = load ptr, ptr %l13
  store ptr %v54, ptr %l15
  %v55 = load ptr, ptr %l11
  store ptr null, ptr %l11
  store ptr %v55, ptr %l16
  %v56 = getelementptr i64, ptr %umask, i64 0
  store i64 0, ptr %v56
  %v57 = load ptr, ptr %l16
  call void @rt_future_suspend(ptr %fut, i32 2, ptr %v57)
  br label %rs2
rs2:
  %v58 = getelementptr i64, ptr %umask, i64 0
  store i64 0, ptr %v58
  %v59 = call ptr @rt_future_resume(ptr %fut)
  %v60 = icmp eq ptr %v59, null
  br i1 %v60, label %pend2, label %ck2
pend2:
  call void @rt_unwind_leave(ptr %urec)
  ret ptr null
ck2:
  %v61 = icmp eq ptr %v59, @kg_cancel_marker
  br i1 %v61, label %can2, label %cont2
can2:
  store ptr null, ptr %l17
  br label %join2
cont2:
  store ptr %v59, ptr %l17
  br label %join2
join2:
  %cf2 = phi i1 [ true, %can2 ], [ false, %cont2 ]
  %v62 = zext i1 %cf2 to i32
  %v63 = call ptr @rt_bool(i32 %v62)
  store ptr %v63, ptr %l18
  %v64 = load ptr, ptr %l18
  %v65 = call i1 @rt_truth(ptr %v64)
  br i1 %v65, label %b3, label %b4
b3:
  %v66 = getelementptr i64, ptr %umask, i64 0
  store i64 0, ptr %v66
  %v67 = load ptr, ptr %l17
  call void @rt_release(ptr %v67)
  %v68 = load ptr, ptr %l15
  store ptr null, ptr %l15
  call void @rt_drop(ptr %v68)
  br label %b1
b4:
  %v69 = load ptr, ptr %l17
  store ptr %v69, ptr %l19
  %v70 = getelementptr i64, ptr %umask, i64 0
  store i64 0, ptr %v70
  %v71 = load ptr, ptr %l15
  %v72 = load ptr, ptr %l19
  %v73 = getelementptr ptr, ptr %v7, i32 0
  store ptr %v71, ptr %v73
  %v74 = getelementptr ptr, ptr %v7, i32 1
  store ptr @.p17, ptr %v74
  %v75 = getelementptr ptr, ptr %v7, i32 2
  store ptr %v72, ptr %v75
  %v76 = call ptr @rt_interp(i32 3, ptr %v7)
  store ptr %v76, ptr %l20
  %v77 = getelementptr i64, ptr %umask, i64 0
  store i64 512, ptr %v77
  %v78 = load ptr, ptr %l20
  %v79 = getelementptr ptr, ptr %v8, i32 0
  store ptr %v78, ptr %v79
  %v80 = call ptr @rt_builtin(ptr @.p11, i32 1, ptr %v8)
  store ptr %v80, ptr %l21
  %v81 = getelementptr i64, ptr %umask, i64 0
  store i64 0, ptr %v81
  %v82 = load ptr, ptr %l20
  call void @rt_release(ptr %v82)
  %v83 = load ptr, ptr %l21
  call void @rt_drop(ptr %v83)
  %v84 = load ptr, ptr %l15
  %v85 = call i64 @rt_int_val(ptr %v84)
  %v86 = load ptr, ptr %l19
  %v87 = call i64 @rt_int_val(ptr %v86)
  %v88 = call { i64, i1 } @llvm.sadd.with.overflow.i64(i64 %v85, i64 %v87)
  %v89 = extractvalue { i64, i1 } %v88, 0
  %v90 = extractvalue { i64, i1 } %v88, 1
  br i1 %v90, label %ovf91, label %ovfok92
ovf91:
  call void @rt_panic(ptr @.p12)
  unreachable
ovfok92:
  %v91 = call ptr @rt_int(i64 %v89, i32 320)
  store ptr %v91, ptr %l22
  %v92 = call ptr @rt_task_close(ptr %fut)
  store ptr %v92, ptr %l23
  %v93 = load ptr, ptr %l19
  store ptr null, ptr %l19
  call void @rt_drop(ptr %v93)
  %v94 = load ptr, ptr %l15
  store ptr null, ptr %l15
  call void @rt_drop(ptr %v94)
  %v95 = load ptr, ptr %l22
  %v96 = call ptr @rt_future_complete(ptr %fut, ptr %v95)
  call void @rt_unwind_leave(ptr %urec)
  %v97 = load ptr, ptr %l22
  ret ptr %v97
b5:
  %v98 = getelementptr i64, ptr %umask, i64 0
  store i64 0, ptr %v98
  %v99 = load ptr, ptr %l13
  call void @rt_release(ptr %v99)
  br label %b1
}
define internal void @"golden.root$framedrop"(ptr %fut) {
entry:
  %slots = call ptr @rt_future_slots(ptr %fut)
  %st = call i32 @rt_future_state(ptr %fut)
  switch i32 %st, label %done [ i32 1, label %d1 i32 2, label %d2 i32 1073741824, label %dp ]
d1:
  %v1 = getelementptr ptr, ptr %slots, i64 5
  %v2 = load ptr, ptr %v1
  store ptr null, ptr %v1
  call void @rt_release(ptr %v2)
  %v3 = getelementptr ptr, ptr %slots, i64 4
  %v4 = load ptr, ptr %v3
  store ptr null, ptr %v3
  call void @rt_release(ptr %v4)
  br label %done
d2:
  %v5 = getelementptr ptr, ptr %slots, i64 7
  %v6 = load ptr, ptr %v5
  store ptr null, ptr %v5
  call void @rt_release(ptr %v6)
  %v7 = getelementptr ptr, ptr %slots, i64 6
  %v8 = load ptr, ptr %v7
  store ptr null, ptr %v7
  call void @rt_release(ptr %v8)
  %v9 = getelementptr ptr, ptr %slots, i64 4
  %v10 = load ptr, ptr %v9
  store ptr null, ptr %v9
  call void @rt_release(ptr %v10)
  br label %done
dp:
  %v11 = getelementptr ptr, ptr %slots, i64 8
  %v12 = load ptr, ptr %v11
  store ptr null, ptr %v11
  call void @rt_release(ptr %v12)
  %v13 = getelementptr ptr, ptr %slots, i64 6
  %v14 = load ptr, ptr %v13
  store ptr null, ptr %v13
  call void @rt_release(ptr %v14)
  %v15 = getelementptr ptr, ptr %slots, i64 4
  %v16 = load ptr, ptr %v15
  store ptr null, ptr %v15
  call void @rt_release(ptr %v16)
  %v17 = getelementptr ptr, ptr %slots, i64 1
  %v18 = load ptr, ptr %v17
  store ptr null, ptr %v17
  call void @rt_release(ptr %v18)
  br label %done
done:
  ret void
}
define ptr @"kigumi.main"() {
entry:
  %urec = alloca [9 x i64], align 8
  %umask = getelementptr i64, ptr %urec, i64 3
  call void @llvm.memset.p0.i64(ptr %umask, i8 0, i64 8, i1 false)
  %l0 = alloca ptr
  store ptr null, ptr %l0
  %l1 = getelementptr i64, ptr %urec, i64 4
  store ptr null, ptr %l1
  %l2 = getelementptr i64, ptr %urec, i64 5
  store ptr null, ptr %l2
  %l3 = getelementptr i64, ptr %urec, i64 6
  store ptr null, ptr %l3
  %l4 = getelementptr i64, ptr %urec, i64 7
  store ptr null, ptr %l4
  %l5 = getelementptr i64, ptr %urec, i64 8
  store ptr null, ptr %l5
  %l6 = alloca ptr
  store ptr null, ptr %l6
  %l7 = alloca ptr
  store ptr null, ptr %l7
  %v1 = alloca ptr, i32 1
  %v2 = alloca ptr, i32 1
  %v3 = alloca ptr, i32 1
  %v4 = alloca ptr, i32 2
  %v5 = alloca ptr, i32 2
  %v6 = alloca ptr, i32 1
  call void @rt_unwind_enter(ptr %urec, i64 5)
  br label %b0
b0:
  %v7 = getelementptr i64, ptr %umask, i64 0
  store i64 0, ptr %v7
  %v8 = call ptr @rt_std_id(i32 53, i32 0, ptr %v1)
  store ptr %v8, ptr %l0
  %v9 = load ptr, ptr %l0
  store ptr %v9, ptr %l1
  store ptr @"golden.root$async.fnitem.async", ptr %l3
  %v10 = getelementptr i64, ptr %umask, i64 0
  store i64 5, ptr %v10
  %v11 = load ptr, ptr %l3
  %v12 = call ptr @rt_future_new(ptr %v11, i32 0, ptr %v3)
  store ptr %v12, ptr %l4
  %v13 = getelementptr i64, ptr %umask, i64 0
  store i64 9, ptr %v13
  %v14 = load ptr, ptr %l3
  %v15 = call i32 @rt_release_outer(ptr %v14)
  %v16 = icmp ne i32 %v15, 0
  br i1 %v16, label %ufail4, label %ucont14
ucont14:
  %v17 = load ptr, ptr %l1
  %v18 = load ptr, ptr %l4
  %v19 = getelementptr ptr, ptr %v4, i32 0
  store ptr %v17, ptr %v19
  %v20 = getelementptr ptr, ptr %v4, i32 1
  store ptr %v18, ptr %v20
  %v21 = call ptr @rt_std_id(i32 54, i32 2, ptr %v4)
  store ptr %v21, ptr %l5
  %v22 = getelementptr i64, ptr %umask, i64 0
  store i64 17, ptr %v22
  %v23 = load ptr, ptr %l4
  %v24 = call i32 @rt_release_outer(ptr %v23)
  %v25 = icmp ne i32 %v24, 0
  br i1 %v25, label %ufail3, label %ucont25
ucont25:
  %v26 = load ptr, ptr %l5
  %v27 = getelementptr ptr, ptr %v5, i32 0
  store ptr @.p18, ptr %v27
  %v28 = getelementptr ptr, ptr %v5, i32 1
  store ptr %v26, ptr %v28
  %v29 = call ptr @rt_interp(i32 2, ptr %v5)
  store ptr %v29, ptr %l2
  %v30 = getelementptr i64, ptr %umask, i64 0
  store i64 3, ptr %v30
  %v31 = load ptr, ptr %l5
  %v32 = call i32 @rt_release_outer(ptr %v31)
  %v33 = icmp ne i32 %v32, 0
  br i1 %v33, label %ufail2, label %ucont35
ucont35:
  %v34 = load ptr, ptr %l2
  %v35 = getelementptr ptr, ptr %v6, i32 0
  store ptr %v34, ptr %v35
  %v36 = call ptr @rt_builtin(ptr @.p11, i32 1, ptr %v6)
  store ptr %v36, ptr %l6
  %v37 = getelementptr i64, ptr %umask, i64 0
  store i64 1, ptr %v37
  %v38 = load ptr, ptr %l2
  %v39 = call i32 @rt_release_outer(ptr %v38)
  %v40 = icmp ne i32 %v39, 0
  br i1 %v40, label %ufail1, label %ucont44
ucont44:
  %v41 = load ptr, ptr %l6
  %v42 = call i32 @rt_release_outer(ptr %v41)
  %v43 = icmp ne i32 %v42, 0
  br i1 %v43, label %ufail1, label %ucont48
ucont48:
  %v44 = getelementptr i64, ptr %umask, i64 0
  store i64 0, ptr %v44
  %v45 = load ptr, ptr %l1
  store ptr null, ptr %l1
  call void @rt_drop_outer(ptr %v45)
  %v46 = call ptr @rt_unit()
  store ptr %v46, ptr %l7
  call void @rt_unwind_leave(ptr %urec)
  %v47 = load ptr, ptr %l7
  ret ptr %v47
ufail1:
  %v48 = load ptr, ptr %l1
  call void @rt_sweep_release(ptr %v48)
  call void @rt_outer_fatal()
  unreachable
ufail2:
  %v49 = load ptr, ptr %l2
  call void @rt_sweep_release(ptr %v49)
  %v50 = load ptr, ptr %l1
  call void @rt_sweep_release(ptr %v50)
  call void @rt_outer_fatal()
  unreachable
ufail3:
  %v51 = load ptr, ptr %l5
  call void @rt_sweep_release(ptr %v51)
  %v52 = load ptr, ptr %l1
  call void @rt_sweep_release(ptr %v52)
  call void @rt_outer_fatal()
  unreachable
ufail4:
  %v53 = load ptr, ptr %l4
  call void @rt_sweep_release(ptr %v53)
  %v54 = load ptr, ptr %l1
  call void @rt_sweep_release(ptr %v54)
  call void @rt_outer_fatal()
  unreachable
}
define void @kg_init_statics() {
  ret void
}
define void @kg_fini_statics() {
  ret void
}
define i32 @main(i32 %argc, ptr %argv) {
  call void @rt_abi_check(ptr @.p19)
  call void @kg_init_immortals()
  call void @rt_init(i32 %argc, ptr %argv, i32 5, ptr @rt_types)
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
@.p6 = private constant [8 x i8] c".Future\00"
@.p7 = private constant [18 x i8] c"std/task.Executor\00"
@.p8 = private constant [14 x i8] c"std/task.Task\00"
@.p9 = private constant [40 x i8] c"this async body cannot resume from here\00"
@.p10 = internal global [64 x i8] zeroinitializer, align 16
@.p11 = private constant [6 x i8] c"print\00"
@.p12 = private constant [17 x i8] c"integer overflow\00"
@.p13 = internal global [64 x i8] zeroinitializer, align 16
@.p14 = internal global [64 x i8] zeroinitializer, align 16
@.p15 = internal global [64 x i8] zeroinitializer, align 16
@.p16 = internal global [64 x i8] zeroinitializer, align 16
@.p17 = internal global [64 x i8] zeroinitializer, align 16
@.p18 = internal global [64 x i8] zeroinitializer, align 16
@.p19 = private constant [40 x i8] c"kgabi1:<hash>\00"
