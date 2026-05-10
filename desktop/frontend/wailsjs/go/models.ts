export namespace main {
	
	export class SystemInfoJSON {
	    os: string;
	    arch: string;
	    cpuModel: string;
	    cpuCores: number;
	    ramBytes: number;
	    ramFormatted: string;
	    diskModel: string;
	    diskSizeBytes: number;
	    diskFreeBytes: number;
	    diskSize: string;
	    diskFree: string;
	
	    static createFrom(source: any = {}) {
	        return new SystemInfoJSON(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.os = source["os"];
	        this.arch = source["arch"];
	        this.cpuModel = source["cpuModel"];
	        this.cpuCores = source["cpuCores"];
	        this.ramBytes = source["ramBytes"];
	        this.ramFormatted = source["ramFormatted"];
	        this.diskModel = source["diskModel"];
	        this.diskSizeBytes = source["diskSizeBytes"];
	        this.diskFreeBytes = source["diskFreeBytes"];
	        this.diskSize = source["diskSize"];
	        this.diskFree = source["diskFree"];
	    }
	}
	export class UpdateInfo {
	    available: boolean;
	    latest: string;
	    current: string;
	    url: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.latest = source["latest"];
	        this.current = source["current"];
	        this.url = source["url"];
	    }
	}

}

