export namespace ui {
	
	export class UISettings {
	    out_dir: string;
	
	    static createFrom(source: any = {}) {
	        return new UISettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.out_dir = source["out_dir"];
	    }
	}

}

