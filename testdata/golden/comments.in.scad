/*[Dimensions]*/
// Width of the box
width = 10; // [1:100]
/* Height */ height = 20;

/**
   * Doc comment
       * more
 */
module m(a, // first
         b = 2, /* second */
         c) {
  // only a comment
}
module empty() {

}
module e2() { // note
}
cube([1, // x
      2,
      3]);
f(a /* c */, b);
x = [
  // leading
  1, 2,
  // before close
];
translate([1,2,3]) // moves it
  cube(1);
if (a) { // why
  b();
}
// before else
else { c(); }
translate([1,0,0]) // c1
rotate(90) cube(1);
y = 1 + // plus
    2;
{
}
// end of file
