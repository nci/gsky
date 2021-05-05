
#include "coordinate_transform_cache.hxx"
#include <iostream>

void CoordinateTransformCache::put(TransformKey key, void* psInfo) {
	remove(key);
	if( coordLookup.size() >= maxCapacity ) {
		int minCount = std::numeric_limits<int>::max();
		auto minIt = coordLookup.begin();
		for(auto it = coordLookup.begin(); it != coordLookup.end(); it++ ) {
			if( it->second->useCount < minCount ) {
				minCount = it->second->useCount;
				minIt = it;
			}
		}
		remove(minIt->first);
	}

	coordLookup[key] = new CacheBlock { psInfo, 1 };
}	

void* CoordinateTransformCache::get(TransformKey key) {
	auto it = coordLookup.find(key);
	if( it != coordLookup.end() ) {
		it->second->useCount++;
		//std::cerr << "xxxxxxxxx " << it->second->useCount << std::endl;
		return it->second->item;
	}
	return nullptr;
}

void CoordinateTransformCache::remove(TransformKey key) {
	auto it = coordLookup.find(key);
	if( it != coordLookup.end() ) {
		GDALDestroyReprojectionTransformer(it->second->item);
		delete it->second;
		coordLookup.erase(it);
	}
}
