var hooks = require('hooks');
var http = require('http');
var fs = require('fs');

const defaultMapping = './service/test/mapping.json';
const indexSettings = '{"index":{"number_of_replicas":0}}' // zero replicas, so ES cluster appears healthy
const donaldTrump = '{"types":["http://www.ft.com/ontology/core/Thing","http://www.ft.com/ontology/concept/Concept","http://www.ft.com/ontology/person/Person"],"aliases":["Donald John Trump","Donald John Trump, Sr.","Donald Trump","Donald John Trump Sr."],"apiUrl":"http://api.ft.com/people/e739b9f1-92d7-42c1-ac16-2ad7697ee5c4","directType":"http://www.ft.com/ontology/person/Person","prefLabel":"Donald John Trump","id":"http://api.ft.com/things/e739b9f1-92d7-42c1-ac16-2ad7697ee5c4","isFTAuthor":"false"}'

hooks.beforeAll(function(t, done) {
   if(!fs.existsSync(defaultMapping)){
      console.log('No mappings found, skipping hook.');
      done();
      return;
   }

   writeMapping();
   writeDonaldTrump();

   setTimeout(()=>{
      updateIndexSettings();
      done();
   }, 5000);
});

var writeMapping = function(callback){
   // Write the test mappings
   var contents = fs.readFileSync(defaultMapping, 'utf8');

   var options = {
      host: 'localhost',
      port: '9200',
      path: '/concepts?wait_for_active_shards=2',
      method: 'PUT',
      headers: {
         'Content-Type': 'application/json'
      }
   };

   var req = http.request(options, function(res) {
      res.setEncoding('utf8');
   });

   req.write(contents);
   req.end(callback);
};

var updateIndexSettings = function(callback){
   // Remove the replicas so the cluster appears healthy with one node
   var options = {
      host: 'localhost',
      port: '9200',
      path: '/concepts/_settings',
      method: 'PUT',
      headers: {
         'Content-Type': 'application/json'
      }
   };

   var req = http.request(options, function(res) {
      res.setEncoding('utf8');
   });

   req.write(indexSettings);
   req.end(callback);
};

var writeDonaldTrump = function(callback){
   // Write Donald Trump as a concept
   var options = {
      host: 'localhost',
      port: '9200',
      path: '/concepts/people/e739b9f1-92d7-42c1-ac16-2ad7697ee5c4?refresh=wait_for',
      method: 'PUT',
      headers: {
         'Content-Type': 'application/json'
      }
   };

   var req = http.request(options, function(res) {
      res.setEncoding('utf8');
   });

   req.write(donaldTrump);
   req.end(callback);
};
