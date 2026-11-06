// header comment

use <lib/foo.scad>
include <bar.scad>
$fn = 64;
width = 10;
height = 20; // trailing
module bracket(w = 10, h = 20, t = 2, center = false) {
  difference() {
    cube([w, h, t], center = center);
    translate([w / 2, h / 2, -1]) cylinder(d = w / 2, h = t + 2);
    /* dangling */
  }
}
function area(w, h) = w * h;
function longish(a, b, c) =
  let (x = a * 2, y = b * 3)
  x + y + c * some_function_name(a, b, c) + another_function_name(c, b, a) + more;
translate([10, 20, 30])
  rotate([0, 0, 90])
    scale([1, 2, 3])
      color("red")
        cube([10, 20, 30], center = true);
translate([1, 0, 0]) rotate(90) {
  cube(1);
  sphere(2);
}
if (a > 1) cube(1);
else if (b) sphere(1);
else {
  cylinder();
}
points = [[0, 0], [10, 0], [10, 10], [0, 10]];
v = [for (i = [0:10]) if (i % 2 == 0) i * 2 else -i];
r = [0 : n - 1];
t = a ? b : c ? d : e;
#cube(1);
%sphere(1);
for (i = [0:5], j = [1:2]) translate([i, j, 0]) cube(1);
x = very_long_function_name(argument_number_one, argument_number_two, argument_number_three, four);
