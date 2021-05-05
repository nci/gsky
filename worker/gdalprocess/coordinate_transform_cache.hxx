#ifndef COORD_TRANSFORM_H
#define COORD_TRANSFORM_H

#include "gdal_alg.h"
#include <map>
#include <limits>
#include <memory>
#include <utility>

typedef std::pair<std::string, std::string> TransformKey;

struct CacheBlock {
	void* item;
	int   useCount;
	CacheBlock(void* item) {
		this->item = item;
		this->useCount = 1;
	}
	~CacheBlock() {
		if( item != nullptr ) {
			GDALDestroyGenImgProjTransformer(item);
		}
	}
};

class CoordinateTransformCache {
	public:
		void put(TransformKey, void* psInfo);
		void* get(TransformKey key);
		void remove(TransformKey key);
	private:
		std::map<TransformKey, std::unique_ptr<CacheBlock>> coordLookup;
		size_t maxCapacity = 1024;
};

#endif
