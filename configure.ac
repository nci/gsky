dnl A skeletal configure script for GSKY.

AC_INIT(GSKY, 1.0)

AC_ARG_WITH([gdal],
	    [AS_HELP_STRING([--with-gdal@<:@=DIR@:>@],
            [location of GDAL library @<:@default: /usr/local@:>@])],
            [],
            [with_gdal=yes])

AS_IF([test "x$with_gdal" != xyes],
	    AC_SUBST(CGO_CFLAGS, "-I$with_gdal/include -L$with_gdal/lib"))

AC_OUTPUT(Makefile)
