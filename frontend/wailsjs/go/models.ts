export namespace app {
	
	export class StartupStep {
	    id: string;
	    title: string;
	    status: string;
	    detail?: string;
	
	    static createFrom(source: any = {}) {
	        return new StartupStep(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.status = source["status"];
	        this.detail = source["detail"];
	    }
	}
	export class StartupStatus {
	    state: string;
	    url: string;
	    port: number;
	    logPath: string;
	    error?: string;
	    steps: StartupStep[];
	
	    static createFrom(source: any = {}) {
	        return new StartupStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.url = source["url"];
	        this.port = source["port"];
	        this.logPath = source["logPath"];
	        this.error = source["error"];
	        this.steps = this.convertValues(source["steps"], StartupStep);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

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

