translate([1,0,0]) rotate(90) { cube(1); sphere(2); }
translate([1,0,0]) rotate(90) translate([1,0,0]) rotate(90) translate([1,0,0]) rotate(90) translate([1,0,0]) { cube(1); sphere(2); }
translate([1,0,0]) if (a) { cube(1); }
if (a) translate([1,0,0]) cube(1);
if (a) { x(); } else y();
