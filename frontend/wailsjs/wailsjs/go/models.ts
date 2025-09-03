export namespace ui {
	
	export class UISettings {
	    out_dir: string;
	    use_local_ai: boolean;
	    llama_temp: number;
	    llama_context: number;
	    llama_model: string;
	    llama_api_key: string;
	
	    static createFrom(source: any = {}) {
	        return new UISettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.out_dir = source["out_dir"];
	        this.use_local_ai = source["use_local_ai"];
	        this.llama_temp = source["llama_temp"];
	        this.llama_context = source["llama_context"];
	        this.llama_model = source["llama_model"];
	        this.llama_api_key = source["llama_api_key"];
	    }
	}

}

