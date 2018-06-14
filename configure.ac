dnl A skeletal configure script for GSKY.

AC_INIT(GSKY, 1.0)

AC_ARG_WITH([gdal],
	    [AS_HELP_STRING([--with-gdal@<:@=DIR@:>@],
            [location of GDAL library @<:@default: /usr@:>@])],
            [],
            [with_gdal=/usr])

CGO_CFLAGS="-I$with_gdal/include -L$with_gdal/lib -Wl,-rpath-link=$with_gdal/lib"
AC_SUBST(CGO_CFLAGS)

AC_OUTPUT(Makefile)
