export namespace harness {
	
	export class Status {
	    state: string;
	    url: string;
	    port: number;
	    logPath: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.url = source["url"];
	        this.port = source["port"];
	        this.logPath = source["logPath"];
	        this.error = source["error"];
	    }
	}

}

