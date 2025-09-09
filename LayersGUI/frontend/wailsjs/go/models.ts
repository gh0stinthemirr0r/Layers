export namespace common {
	
	export class TestMetrics {
	    duration: number;
	    transfer_rate: number;
	    latency: number;
	    packet_loss: number;
	    response_time: number;
	    jitter: number;
	    reliability_pct: number;
	    custom?: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new TestMetrics(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.duration = source["duration"];
	        this.transfer_rate = source["transfer_rate"];
	        this.latency = source["latency"];
	        this.packet_loss = source["packet_loss"];
	        this.response_time = source["response_time"];
	        this.jitter = source["jitter"];
	        this.reliability_pct = source["reliability_pct"];
	        this.custom = source["custom"];
	    }
	}
	export class TestResult {
	    layer: number;
	    name: string;
	    status: string;
	    message: string;
	    // Go type: time
	    start_time: any;
	    // Go type: time
	    end_time: any;
	    metrics: TestMetrics;
	    sub_results?: TestResult[];
	    diagnostics?: any;
	
	    static createFrom(source: any = {}) {
	        return new TestResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.layer = source["layer"];
	        this.name = source["name"];
	        this.status = source["status"];
	        this.message = source["message"];
	        this.start_time = this.convertValues(source["start_time"], null);
	        this.end_time = this.convertValues(source["end_time"], null);
	        this.metrics = this.convertValues(source["metrics"], TestMetrics);
	        this.sub_results = this.convertValues(source["sub_results"], TestResult);
	        this.diagnostics = source["diagnostics"];
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

export namespace main {
	
	export class NetworkDetails {
	    interfaceName: string;
	    status: string;
	    ipv4Address: string[];
	    ipv6Address: string[];
	    isPrimary: boolean;
	    isVPN: boolean;
	
	    static createFrom(source: any = {}) {
	        return new NetworkDetails(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.interfaceName = source["interfaceName"];
	        this.status = source["status"];
	        this.ipv4Address = source["ipv4Address"];
	        this.ipv6Address = source["ipv6Address"];
	        this.isPrimary = source["isPrimary"];
	        this.isVPN = source["isVPN"];
	    }
	}
	export class PortInfo {
	    port: number;
	    protocol: string;
	    service: string;
	    isVulnerable: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PortInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.port = source["port"];
	        this.protocol = source["protocol"];
	        this.service = source["service"];
	        this.isVulnerable = source["isVulnerable"];
	    }
	}
	export class SecurityFindings {
	    networkDetails: NetworkDetails[];
	    openPorts: PortInfo[];
	    vulnerabilities: string[];
	
	    static createFrom(source: any = {}) {
	        return new SecurityFindings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.networkDetails = this.convertValues(source["networkDetails"], NetworkDetails);
	        this.openPorts = this.convertValues(source["openPorts"], PortInfo);
	        this.vulnerabilities = source["vulnerabilities"];
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

