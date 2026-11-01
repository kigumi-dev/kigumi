package mirfixture

const DirectPointSrc = `
type Point = {
    pub x Int
    pub y Int
}

fn local(x: Int) -> Int {
    let p = Point { x: x, y: x }
    p.x + p.y
}

fn intoArray(x: Int) -> Array[Point] {
    Array.of(Point { x: x, y: x })
}

fn main() -> Void! {}
`

const ClosureEnvSrc = `
type Point = {
    pub x Int
    pub y Int
}

fn apply(f: fn(Int) -> Int) -> Int { f(1) }

fn local(n: Int, p: Point) -> Int {
    let f = (x: Int) => x + n + p.y
    f(2)
}

fn erases(n: Int) -> Int {
    apply((x: Int) => x + n)
}

fn lends(p: Point) -> Bool {
    let f = () => hash(&p) == hash(&p)
    f()
}

fn main() -> Void! {}
`

const CrossingSrc = `
type Point = {
    pub x Int
    pub y Int
}

fn take(p: Point) -> Int { p.x }

fn mk(x: Int) -> Point { Point { x: x, y: x } }

fn use(x: Int) -> Int {
    let p = mk(x)
    take(p)
}

fn main() -> Void! {}
`
