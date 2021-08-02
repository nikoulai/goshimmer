(self.webpackChunkdoc_ops=self.webpackChunkdoc_ops||[]).push([[8885],{630:function(e,t,n){"use strict";n.r(t),n.d(t,{frontMatter:function(){return s},contentTitle:function(){return c},metadata:function(){return l},toc:function(){return u},default:function(){return p}});var r=n(2122),o=n(9756),i=(n(7294),n(3905)),a=["components"],s={},c="Create a static identity",l={unversionedId:"tutorials/static_identity",id:"tutorials/static_identity",isDocsHomePage:!1,title:"Create a static identity",description:"To create a static GoShimmer identity, you will need to generate a random 32byte autopeering seed. You can use openssl or the rand-seed tool we provide under the GoShimmer folder tools/rand-seed.",source:"@site/docs/tutorials/static_identity.md",sourceDirName:"tutorials",slug:"/tutorials/static_identity",permalink:"/docs/tutorials/static_identity",editUrl:"https://github.com/iotaledger/Goshimmer/tree/develop/docOps/docs/tutorials/static_identity.md",version:"current",frontMatter:{},sidebar:"docs",previous:{title:"Manual Peering",permalink:"/docs/tutorials/manual_peering"},next:{title:"dRNG API",permalink:"/docs/tutorials/custom_dRNG"}},u=[],d={toc:u};function p(e){var t=e.components,n=(0,o.Z)(e,a);return(0,i.kt)("wrapper",(0,r.Z)({},d,n,{components:t,mdxType:"MDXLayout"}),(0,i.kt)("h1",{id:"create-a-static-identity"},"Create a static identity"),(0,i.kt)("p",null,"To create a static GoShimmer identity, you will need to generate a random 32byte autopeering seed. You can use ",(0,i.kt)("inlineCode",{parentName:"p"},"openssl")," or the ",(0,i.kt)("inlineCode",{parentName:"p"},"rand-seed")," tool we provide under the GoShimmer folder ",(0,i.kt)("inlineCode",{parentName:"p"},"tools/rand-seed"),".\nFor example, by running:"),(0,i.kt)("ul",null,(0,i.kt)("li",{parentName:"ul"},(0,i.kt)("inlineCode",{parentName:"li"},"openssl rand -base64 32"),": generates a random 32 byte sequence encoded in base64. The output should look like: ",(0,i.kt)("inlineCode",{parentName:"li"},"gP0uRLhwBG2yJJmnLySX4S4R5G250Z3dbN9yBR6VSyY=")),(0,i.kt)("li",{parentName:"ul"},(0,i.kt)("inlineCode",{parentName:"li"},"go run main.go")," under the GoShimmer folder ",(0,i.kt)("inlineCode",{parentName:"li"},"tools/rand-seed"),": generates a random 32 byte sequence encoded in both base64 and base58. The output is written into the file ",(0,i.kt)("inlineCode",{parentName:"li"},"random-seed.txt")," and should look like:")),(0,i.kt)("pre",null,(0,i.kt)("code",{parentName:"pre"},"base64:nQW9MhNSLpIqBUiZe90XI320g680zxFoB1UIK09Acus=\nbase58:BZx5tDLymckUV5wiswXJtajgQrBEzTBBRR4uGfr1YNGS\n")),(0,i.kt)("p",null,"You can now copy one of that strings (together with the encoding type prefix) and paste it into the GoShimmer ",(0,i.kt)("inlineCode",{parentName:"p"},"config.json")," under the ",(0,i.kt)("inlineCode",{parentName:"p"},"autopeering")," section:"),(0,i.kt)("pre",null,(0,i.kt)("code",{parentName:"pre"},'"autopeering": {\n    "entryNodes": [\n      "2PV5487xMw5rasGBXXWeqSi4hLz7r19YBt8Y1TGAsQbj@ressims.iota.cafe:15626"\n    ],\n    "port": 14626,\n    "seed":"base64:gP0uRLhwBG2yJJmnLySX4S4R5G250Z3dbN9yBR6VSyY="\n  },\n')),(0,i.kt)("p",null,"Or if you are using docker and prefer to set this with a command, you can define the same by changing the GoShimmer docker-compose.yml:"),(0,i.kt)("pre",null,(0,i.kt)("code",{parentName:"pre",className:"language-yml"},'goshimmer:\n    network_mode: host\n    image: iotaledger/goshimmer\n    build:\n      context: ./\n      dockerfile: Dockerfile\n    container_name: iota_goshimmer\n    command: >\n      --node.enablePlugins=prometheus\n      --autopeering.seed="base64:gP0uRLhwBG2yJJmnLySX4S4R5G250Z3dbN9yBR6VSyY="\n    # Mount volumes:\n    # make sure to give read/write access to the folder ./mainnetdb (e.g., chmod -R 777 ./mainnetdb)\n    # optionally, you can mount a config.json into the container\n    volumes:\n      - ./mainnetdb/:/tmp/mainnetdb/:rw\n      - ./config.json:/config.json:ro\n    # Expose ports:\n    # gossip:       - "14666:14666/tcp"\n    # autopeering:  - "14626:14626/udp"\n    # webAPI:       - "8080:8080/tcp"\n    # dashboard:    - "8081:8081/tcp"\n    ports:\n      - "14666:14666/tcp"\n      - "14626:14626/udp"\n      - "9311:9311/tcp" # prometheus exporter\n      - "8080:8080/tcp" # webApi\n      - "8081:8081/tcp" # dashboard\n')))}p.isMDXComponent=!0},3905:function(e,t,n){"use strict";n.d(t,{Zo:function(){return u},kt:function(){return m}});var r=n(7294);function o(e,t,n){return t in e?Object.defineProperty(e,t,{value:n,enumerable:!0,configurable:!0,writable:!0}):e[t]=n,e}function i(e,t){var n=Object.keys(e);if(Object.getOwnPropertySymbols){var r=Object.getOwnPropertySymbols(e);t&&(r=r.filter((function(t){return Object.getOwnPropertyDescriptor(e,t).enumerable}))),n.push.apply(n,r)}return n}function a(e){for(var t=1;t<arguments.length;t++){var n=null!=arguments[t]?arguments[t]:{};t%2?i(Object(n),!0).forEach((function(t){o(e,t,n[t])})):Object.getOwnPropertyDescriptors?Object.defineProperties(e,Object.getOwnPropertyDescriptors(n)):i(Object(n)).forEach((function(t){Object.defineProperty(e,t,Object.getOwnPropertyDescriptor(n,t))}))}return e}function s(e,t){if(null==e)return{};var n,r,o=function(e,t){if(null==e)return{};var n,r,o={},i=Object.keys(e);for(r=0;r<i.length;r++)n=i[r],t.indexOf(n)>=0||(o[n]=e[n]);return o}(e,t);if(Object.getOwnPropertySymbols){var i=Object.getOwnPropertySymbols(e);for(r=0;r<i.length;r++)n=i[r],t.indexOf(n)>=0||Object.prototype.propertyIsEnumerable.call(e,n)&&(o[n]=e[n])}return o}var c=r.createContext({}),l=function(e){var t=r.useContext(c),n=t;return e&&(n="function"==typeof e?e(t):a(a({},t),e)),n},u=function(e){var t=l(e.components);return r.createElement(c.Provider,{value:t},e.children)},d={inlineCode:"code",wrapper:function(e){var t=e.children;return r.createElement(r.Fragment,{},t)}},p=r.forwardRef((function(e,t){var n=e.components,o=e.mdxType,i=e.originalType,c=e.parentName,u=s(e,["components","mdxType","originalType","parentName"]),p=l(n),m=o,y=p["".concat(c,".").concat(m)]||p[m]||d[m]||i;return n?r.createElement(y,a(a({ref:t},u),{},{components:n})):r.createElement(y,a({ref:t},u))}));function m(e,t){var n=arguments,o=t&&t.mdxType;if("string"==typeof e||o){var i=n.length,a=new Array(i);a[0]=p;var s={};for(var c in t)hasOwnProperty.call(t,c)&&(s[c]=t[c]);s.originalType=e,s.mdxType="string"==typeof e?e:o,a[1]=s;for(var l=2;l<i;l++)a[l]=n[l];return r.createElement.apply(null,a)}return r.createElement.apply(null,n)}p.displayName="MDXCreateElement"}}]);