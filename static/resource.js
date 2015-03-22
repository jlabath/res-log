$(function(){
    //Res model for our resource
    var ResModel = Backbone.Model.extend({
        defaults: function() {
            return {
                fetchdate: "missing date",
                hookdate: "missing date",
                sha1: "missing checksum",
                resource: {}
            };
        }
    });

    var ResourceView = Backbone.View.extend({
        template: _.template($('#res-template').html()),
        render: function() {
            this.$el.html(this.template(this.model.toJSON()));
            return this;
        }
    });

    //our list based on the search
    var ResList = Backbone.Collection.extend({
         model: ResModel,
         comparator: function(x, y){
            if (x < y) {
                return 1;
            } else if (x > y) {
                return -1;
            }
            return 0;
         }
    });
    //ok def resources in here
    var resources = new ResList();
    //our view that listens to changes in resources
    var AppView = Backbone.View.extend({
        el: $("#resapp"),
        events: {
            "change #reslst": "showResource",
            "click #resgo": "fetchResources",
            "keypress #resform": "goOnEnter"
        },
        fetchResources: function(){
            var restype = this.$("#restype");
            var resid = this.$("#resid");
            var url = "/l/"+restype.val()+"/"+$.trim(resid.val());
            resources.url = url;
            resources.fetch({reset:true});
        },
        initialize: function() {
            this.listenTo(resources, 'reset', this.render);
        },
        render: function(){
            var lst = this.$("#reslst");
            lst.empty();
            resources.forEach(function(val){
                lst.append($("<option>").html(val.get("fetchdate")).attr('value',val.cid));
            });
            this.$("#reslst").change();
        },
        showResource: function(){
            var cid = this.$("#reslst").val();
            if (cid){
                var rsc = resources.get(cid);
                var view = new ResourceView({model: rsc});
                this.$("#resview").hide();
                this.$("#resview").empty();
                this.$("#resview").append(view.render().el);
                this.$("#resview").show();
            } else {
                this.$("#resview").hide();
            }
        },
        goOnEnter: function(e){
            if (e.keyCode == 13){
                e.preventDefault();
                this.fetchResources();
                return false;
            }
        }
    });
    //start it
    var app = new AppView();
});
