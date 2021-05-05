#ifndef COORD_TRANSFORM_H
#define COORD_TRANSFORM_H

#include "gdal_alg.h"
#include <map>
#include <limits>
#include <utility>

typedef std::pair<std::string, std::string> TransformKey;

struct CacheBlock {
	void* item;
	int   useCount;
};

class CoordinateTransformCache {
	public:
		void put(TransformKey, void* psInfo);
		void* get(TransformKey key);
		void remove(TransformKey key);
	private:
		std::map<TransformKey, CacheBlock* > coordLookup;
		size_t maxCapacity = 1024;
};

#endif
